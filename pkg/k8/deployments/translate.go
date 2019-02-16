package deployments

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/cloudnativedevelopment/cnd/pkg/log"
	"github.com/cloudnativedevelopment/cnd/pkg/model"
	"github.com/cloudnativedevelopment/cnd/pkg/syncthing"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
)

const (
	cndEnvNamespace  = "CND_KUBERNETES_NAMESPACE"
	initSyncImageTag = "okteto/init-syncthing:0.4.1"
	syncImageTag     = "okteto/syncthing:0.4.0"
)

var (
	devReplicas                      int32 = 1
	devTerminationGracePeriodSeconds int64
	rootUID                          = int64(0)
)

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
			if c.Name == dev.Swap.Deployment.Container {
				updateCndContainer(&d.Spec.Template.Spec.Containers[i], dev, d.Namespace)
				break
			}
		}
		createInitSyncthingContainer(d, dev)
		createSyncthingVolume(d, dev)
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
		c.SecurityContext = &v1.SecurityContext{}
	}

	c.SecurityContext.RunAsUser = &rootUID

	if dev.Swap.Deployment.Image != "" {
		c.Image = dev.Swap.Deployment.Image
	}

	if len(dev.Swap.Deployment.Command) > 0 {
		c.Command = dev.Swap.Deployment.Command
	}
	if len(dev.Swap.Deployment.Args) > 0 {
		c.Args = dev.Swap.Deployment.Args
	}

	c.WorkingDir = dev.Mount.Target
	c.ReadinessProbe = nil
	c.LivenessProbe = nil

	if c.VolumeMounts == nil {
		c.VolumeMounts = []apiv1.VolumeMount{}
	}

	volumeMount := apiv1.VolumeMount{
		Name:      dev.GetCNDSyncVolume(),
		MountPath: dev.Mount.Target,
	}

	c.VolumeMounts = append(
		c.VolumeMounts,
		volumeMount,
	)

	c.Resources = apiv1.ResourceRequirements{}
	mergeEnvironmentVariables(c, dev.Environment, namespace)
}

func mergeEnvironmentVariables(c *v1.Container, devEnv []model.EnvVar, namespace string) {
	devEnv = append(devEnv, model.EnvVar{Name: cndEnvNamespace, Value: namespace})
	unusedDevEnv := map[string]string{}

	for _, val := range devEnv {
		unusedDevEnv[val.Name] = val.Value
	}

	for i, envvar := range c.Env {
		if value, ok := unusedDevEnv[envvar.Name]; ok {
			c.Env[i] = v1.EnvVar{Name: envvar.Name, Value: value}
			delete(unusedDevEnv, envvar.Name)
		}
	}

	for _, envvar := range devEnv {
		if value, ok := unusedDevEnv[envvar.Name]; ok {
			c.Env = append(c.Env, v1.EnvVar{Name: envvar.Name, Value: value})
		}
	}
}

func createInitSyncthingContainer(d *appsv1.Deployment, dev *model.Dev) {
	image := dev.Swap.Deployment.Image
	if image == "" {
		for _, c := range d.Spec.Template.Spec.Containers {
			if c.Name == dev.Swap.Deployment.Container {
				image = c.Image
				break
			}
		}
	}
	source := filepath.Join(dev.Mount.Target, "*")
	initSyncthingContainer := apiv1.Container{
		Name:  dev.GetCNDInitSyncContainer(),
		Image: image,
		VolumeMounts: []apiv1.VolumeMount{
			apiv1.VolumeMount{
				Name:      dev.GetCNDSyncVolume(),
				MountPath: "/cnd/init",
			},
		},
		Command: []string{
			"sh",
			"-c",
			fmt.Sprintf(`[ "$(ls -A /cnd/init)" ] || mv %s /cnd/init || true`, source),
		},
	}

	if d.Spec.Template.Spec.InitContainers == nil {
		d.Spec.Template.Spec.InitContainers = []apiv1.Container{}
	}

	d.Spec.Template.Spec.InitContainers = append(d.Spec.Template.Spec.InitContainers, initSyncthingContainer)
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

	syncVolume := apiv1.Volume{Name: dev.GetCNDSyncVolume()}
	d.Spec.Template.Spec.Volumes = append(
		d.Spec.Template.Spec.Volumes,
		syncVolume,
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
