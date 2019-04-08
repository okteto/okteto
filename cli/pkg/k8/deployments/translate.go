package deployments

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"cli/cnd/pkg/log"
	"cli/cnd/pkg/model"
	"cli/cnd/pkg/syncthing"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	cndEnvNamespace  = "CND_KUBERNETES_NAMESPACE"
	initSyncImageTag = "okteto/init-syncthing:0.4.1"
	syncImageTag     = "okteto/syncthing:0.4.2"
)

var (
	devReplicas                      int32 = 1
	devTerminationGracePeriodSeconds int64
)

// GetSandBoxManifest returns a k8s manifests for a new sandbox deployment
func GetSandBoxManifest(dev *model.Dev, namespace string) *appsv1.Deployment {
	image := "okteto/desk:0.1.2"
	if dev.Image != "" {
		image = dev.Image
	}
	container := "desk"
	if dev.Container != "" {
		container = dev.Container
	}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dev.Name,
			Namespace: namespace,
			Annotations: map[string]string{
				model.CNDAutoDestroyOnDown: "true",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &devReplicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": dev.Name,
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": dev.Name,
					},
				},
				Spec: apiv1.PodSpec{
					TerminationGracePeriodSeconds: &devTerminationGracePeriodSeconds,
					Containers: []apiv1.Container{
						apiv1.Container{
							Name:            container,
							Image:           image,
							ImagePullPolicy: apiv1.PullIfNotPresent,
							Command:         []string{"tail", "-f", "/dev/null"},
						},
					},
				},
			},
		},
	}
}

func translateToDevModeDeployment(d *appsv1.Deployment, devList []*model.Dev) error {

	d.Status = appsv1.DeploymentStatus{}
	manifest, err := json.Marshal(d)
	if err != nil {
		return err
	}
	setAnnotation(d.GetObjectMeta(), model.CNDDeploymentAnnotation, string(manifest))
	if err := setDevListAsAnnotation(d.GetObjectMeta(), devList); err != nil {
		return err
	}
	setLabel(d.GetObjectMeta(), model.CNDLabel, d.Name)
	setLabel(d.Spec.Template.GetObjectMeta(), model.CNDLabel, d.Name)
	d.Spec.Strategy = appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType}
	d.Spec.Template.Spec.TerminationGracePeriodSeconds = &devTerminationGracePeriodSeconds

	for _, dev := range devList {
		for i, c := range d.Spec.Template.Spec.Containers {
			if c.Name == dev.Container {
				updateCndContainer(&d.Spec.Template.Spec.Containers[i], dev, d.Namespace)
				break
			}
		}
		createInitSyncthingContainer(d, dev)
		createSyncthingVolume(d, dev)
		createDataVolumes(d, dev)
		if dev.EnableDocker {
			createDinDContainer(d, dev)
			createDinDVolume(d, dev)
		}
	}

	createSyncthingContainer(d, devList)
	createSyncthingSecretVolume(d)

	if *(d.Spec.Replicas) != devReplicas {
		log.Info("cnd only supports running with 1 replica")
		d.Spec.Replicas = &devReplicas
	}
	return nil
}

func updateCndContainer(c *apiv1.Container, dev *model.Dev, namespace string) {
	if c.SecurityContext == nil {
		c.SecurityContext = &apiv1.SecurityContext{}
	}
	if dev.RunAsUser != nil {
		c.SecurityContext.RunAsUser = dev.RunAsUser
	}
	if dev.Image != "" {
		c.Image = dev.Image
	}

	c.ImagePullPolicy = apiv1.PullAlways
	c.Command = []string{"tail"}
	c.Args = []string{"-f", "/dev/null"}
	c.WorkingDir = dev.WorkDir.Path
	c.ReadinessProbe = nil
	c.LivenessProbe = nil

	if c.VolumeMounts == nil {
		c.VolumeMounts = []apiv1.VolumeMount{}
	}
	volumeMount := apiv1.VolumeMount{
		Name:      dev.GetCNDSyncVolume(),
		MountPath: dev.WorkDir.Path,
	}
	c.VolumeMounts = append(
		c.VolumeMounts,
		volumeMount,
	)
	for _, v := range dev.Volumes {
		volumeMount = apiv1.VolumeMount{
			Name:      dev.GetCNDDataVolume(v),
			MountPath: v.Path,
		}
		c.VolumeMounts = append(
			c.VolumeMounts,
			volumeMount,
		)
	}

	overrideResources(c, &dev.Resources)
	mergeEnvironmentVariables(c, dev.Environment, namespace, dev.EnableDocker)
}

func mergeEnvironmentVariables(c *apiv1.Container, devEnv []model.EnvVar, namespace string, enableDocker bool) {
	devEnv = append(devEnv, model.EnvVar{Name: cndEnvNamespace, Value: namespace})
	if enableDocker {
		devEnv = append(devEnv, model.EnvVar{Name: "DOCKER_HOST", Value: "tcp://localhost:2375"})
	}

	unusedDevEnv := map[string]string{}

	for _, val := range devEnv {
		unusedDevEnv[val.Name] = val.Value
	}

	for i, envvar := range c.Env {
		if value, ok := unusedDevEnv[envvar.Name]; ok {
			c.Env[i] = apiv1.EnvVar{Name: envvar.Name, Value: value}
			delete(unusedDevEnv, envvar.Name)
		}
	}

	for _, envvar := range devEnv {
		if value, ok := unusedDevEnv[envvar.Name]; ok {
			c.Env = append(c.Env, apiv1.EnvVar{Name: envvar.Name, Value: value})
		}
	}
}

func overrideResources(c *apiv1.Container, resources *model.ResourceRequirements) {
	for k, v := range resources.Requests {
		if !v.IsZero() {
			if c.Resources.Requests == nil {
				c.Resources.Requests = make(map[apiv1.ResourceName]resource.Quantity, 0)
			}
			c.Resources.Requests[k] = v
		}
	}

	for k, v := range resources.Limits {
		if !v.IsZero() {
			if c.Resources.Limits == nil {
				c.Resources.Limits = make(map[apiv1.ResourceName]resource.Quantity, 0)
			}

			c.Resources.Limits[k] = v
		}
	}
}

func createInitSyncthingContainer(d *appsv1.Deployment, dev *model.Dev) {
	image := dev.Image
	if image == "" {
		for _, c := range d.Spec.Template.Spec.Containers {
			if c.Name == dev.Container {
				image = c.Image
				break
			}
		}
	}
	source := filepath.Join(dev.WorkDir.Path, "*")
	initSyncthingContainer := apiv1.Container{
		Name:  dev.GetCNDInitSyncContainer(),
		Image: image,
		VolumeMounts: []apiv1.VolumeMount{
			apiv1.VolumeMount{
				Name:      dev.GetCNDSyncVolume(),
				MountPath: "/cnd/init",
			},
		},
	}

	if dev.RunAsUser == nil {
		initSyncthingContainer.Command = []string{
			"sh",
			"-c",
			fmt.Sprintf(`ls -A /cnd/init | grep -v "lost+found" || cp -Rf %s /cnd/init || true`, source),
		}
	} else {
		if initSyncthingContainer.SecurityContext == nil {
			initSyncthingContainer.SecurityContext = &apiv1.SecurityContext{}
		}
		var zero int64
		initSyncthingContainer.SecurityContext.RunAsUser = &zero
		initSyncthingContainer.Command = []string{
			"sh",
			"-c",
			fmt.Sprintf(`ls -A /cnd/init | grep -v "lost+found" || (cp -Rf %s /cnd/init; chown -R %d:%d /cnd/init)`, source, *dev.RunAsUser, *dev.RunAsUser),
		}
	}

	if d.Spec.Template.Spec.InitContainers == nil {
		d.Spec.Template.Spec.InitContainers = []apiv1.Container{}
	}

	d.Spec.Template.Spec.InitContainers = append(d.Spec.Template.Spec.InitContainers, initSyncthingContainer)
}

func createDinDContainer(d *appsv1.Deployment, dev *model.Dev) {
	privileged := true
	dindContainer := apiv1.Container{
		Name:            dev.GetCNDDinDContainer(),
		Image:           "docker:18.09.1-dind",
		SecurityContext: &apiv1.SecurityContext{Privileged: &privileged},
		VolumeMounts: []apiv1.VolumeMount{
			apiv1.VolumeMount{
				Name:      dev.GetCNDDinDVolume(),
				MountPath: "/var/lib/docker",
			},
			apiv1.VolumeMount{
				Name:      dev.GetCNDSyncVolume(),
				MountPath: dev.WorkDir.Path,
			},
		},
	}
	d.Spec.Template.Spec.Containers = append(d.Spec.Template.Spec.Containers, dindContainer)
}

func createSyncthingContainer(d *appsv1.Deployment, devList []*model.Dev) {
	syncthingContainer := apiv1.Container{
		Name:            model.CNDSyncContainer,
		Image:           syncImageTag,
		ImagePullPolicy: apiv1.PullIfNotPresent,
		VolumeMounts: []apiv1.VolumeMount{
			apiv1.VolumeMount{
				Name:      model.CNDSyncSecretVolume,
				MountPath: "/var/syncthing/secret/",
			},
		},
		Ports: []apiv1.ContainerPort{
			apiv1.ContainerPort{
				ContainerPort: 8384,
			},
			apiv1.ContainerPort{
				ContainerPort: syncthing.ClusterPort,
			},
		},

		SecurityContext: &apiv1.SecurityContext{},
	}

	user := devList[len(devList)-1].RunAsUser

	if user != nil {
		syncthingContainer.SecurityContext.RunAsUser = user
	}

	for _, dev := range devList {
		syncthingContainer.VolumeMounts = append(
			syncthingContainer.VolumeMounts,
			apiv1.VolumeMount{
				Name:      dev.GetCNDSyncVolume(),
				MountPath: dev.GetCNDSyncMount(),
			},
		)
	}
	d.Spec.Template.Spec.Containers = append(d.Spec.Template.Spec.Containers, syncthingContainer)
}

func createSyncthingVolume(d *appsv1.Deployment, dev *model.Dev) {
	if d.Spec.Template.Spec.Volumes == nil {
		d.Spec.Template.Spec.Volumes = []apiv1.Volume{}
	}

	for _, v := range d.Spec.Template.Spec.Volumes {
		if v.Name == dev.GetCNDSyncVolume() {
			return
		}
	}
	syncVolume := apiv1.Volume{
		Name: dev.GetCNDSyncVolume(),
		VolumeSource: apiv1.VolumeSource{
			PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
				ClaimName: dev.GetCNDSyncVolume(),
				ReadOnly:  false,
			},
		},
	}
	d.Spec.Template.Spec.Volumes = append(
		d.Spec.Template.Spec.Volumes,
		syncVolume,
	)
}

func createDataVolumes(d *appsv1.Deployment, dev *model.Dev) {
	if d.Spec.Template.Spec.Volumes == nil {
		d.Spec.Template.Spec.Volumes = []apiv1.Volume{}
	}

	for _, dataV := range dev.Volumes {
		for _, v := range d.Spec.Template.Spec.Volumes {
			if v.Name == dev.GetCNDDataVolume(dataV) {
				continue
			}
		}
		dataVolume := apiv1.Volume{
			Name: dev.GetCNDDataVolume(dataV),
			VolumeSource: apiv1.VolumeSource{
				PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
					ClaimName: dev.GetCNDDataVolume(dataV),
					ReadOnly:  false,
				},
			},
		}
		d.Spec.Template.Spec.Volumes = append(
			d.Spec.Template.Spec.Volumes,
			dataVolume,
		)
	}
}

func createDinDVolume(d *appsv1.Deployment, dev *model.Dev) {
	if d.Spec.Template.Spec.Volumes == nil {
		d.Spec.Template.Spec.Volumes = []apiv1.Volume{}
	}
	dindVolume := apiv1.Volume{
		Name: dev.GetCNDDinDVolume(),
		VolumeSource: apiv1.VolumeSource{
			PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
				ClaimName: dev.GetCNDDinDVolume(),
				ReadOnly:  false,
			},
		},
	}
	d.Spec.Template.Spec.Volumes = append(
		d.Spec.Template.Spec.Volumes,
		dindVolume,
	)
}

func createSyncthingSecretVolume(d *appsv1.Deployment) {
	if d.Spec.Template.Spec.Volumes == nil {
		d.Spec.Template.Spec.Volumes = []apiv1.Volume{}
	}

	syncVolume := apiv1.Volume{
		Name: model.CNDSyncSecretVolume,
		VolumeSource: apiv1.VolumeSource{
			Secret: &apiv1.SecretVolumeSource{
				SecretName: model.GetCNDSyncSecret(d.Name),
			},
		},
	}

	d.Spec.Template.Spec.Volumes = append(
		d.Spec.Template.Spec.Volumes,
		syncVolume,
	)
}
