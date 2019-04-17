package deployments

import (
	"fmt"
	"path/filepath"

	"github.com/okteto/app/api/model"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	initSyncImageTag    = "okteto/init-syncthing:0.4.1"
	syncImageTag        = "okteto/syncthing:0.4.2"
	syncTCPPort         = 22000
	syncGUIPort         = 8384
	oktetoLabel         = "dev.okteto.com"
	oktetoContainer     = "okteto"
	oktetoSecretVolume  = "okteto-secret"
	oktetoInitContainer = "okteto-init"
	oktetoMount         = "/var/okteto"
)

var (
	devReplicas                      int32 = 1
	devTerminationGracePeriodSeconds int64
)

func devSandbox(dev *model.Dev, s *model.Space) *appsv1.Deployment {
	noServiceAccountToken := false
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dev.Name,
			Namespace: s.ID,
			Labels: map[string]string{
				oktetoLabel: dev.Name,
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
						"app":       dev.Name,
						oktetoLabel: dev.Name,
					},
				},
				Spec: apiv1.PodSpec{
					AutomountServiceAccountToken:  &noServiceAccountToken,
					TerminationGracePeriodSeconds: &devTerminationGracePeriodSeconds,
					Containers: []apiv1.Container{
						apiv1.Container{
							Name:            "dev",
							Image:           dev.Image,
							ImagePullPolicy: apiv1.PullIfNotPresent,
							Command:         []string{"tail", "-f", "/dev/null"},
						},
					},
				},
			},
		},
	}
}

func translate(dev *model.Dev, s *model.Space) *appsv1.Deployment {
	d := devSandbox(dev, s)
	d.Spec.Strategy = appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType}
	d.Spec.Template.Spec.TerminationGracePeriodSeconds = &devTerminationGracePeriodSeconds
	d.Spec.Replicas = &devReplicas
	for i := range d.Spec.Template.Spec.Containers {
		translateDevContainer(&d.Spec.Template.Spec.Containers[i], dev, s)
	}
	translateInitOktetoContainer(d, dev)
	translateOktetoVolume(d, dev)
	translateOktetoContainer(d, dev)
	translateOktetoSecretVolume(d, dev)
	return d
}

func translateDevContainer(c *apiv1.Container, dev *model.Dev, s *model.Space) {
	c.SecurityContext = &apiv1.SecurityContext{}
	c.Image = dev.Image
	c.ImagePullPolicy = apiv1.PullAlways
	c.Command = []string{"tail"}
	c.Args = []string{"-f", "/dev/null"}
	c.WorkingDir = dev.WorkDir
	c.ReadinessProbe = nil
	c.LivenessProbe = nil

	c.VolumeMounts = []apiv1.VolumeMount{
		apiv1.VolumeMount{
			Name:      dev.GetVolumeName(),
			MountPath: dev.WorkDir,
		},
	}
	translateResources(c)
	translateEnvVars(c, dev.Environment)
}

func translateResources(c *apiv1.Container) {
	c.Resources.Requests = make(map[apiv1.ResourceName]resource.Quantity, 0)
	parsed, _ := resource.ParseQuantity("0.250Gi")
	c.Resources.Requests[apiv1.ResourceMemory] = parsed
	parsed, _ = resource.ParseQuantity("0.125")
	c.Resources.Requests[apiv1.ResourceCPU] = parsed
	c.Resources.Limits = make(map[apiv1.ResourceName]resource.Quantity, 0)
	parsed, _ = resource.ParseQuantity("1Gi")
	c.Resources.Limits[apiv1.ResourceMemory] = parsed
	parsed, _ = resource.ParseQuantity("0.5")
	c.Resources.Limits[apiv1.ResourceCPU] = parsed
}

func translateEnvVars(c *apiv1.Container, devEnv []model.EnvVar) {
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

func translateInitOktetoContainer(d *appsv1.Deployment, dev *model.Dev) {
	source := filepath.Join(dev.WorkDir, "*")
	c := apiv1.Container{
		Name:  oktetoInitContainer,
		Image: dev.Image,
		VolumeMounts: []apiv1.VolumeMount{
			apiv1.VolumeMount{
				Name:      dev.GetVolumeName(),
				MountPath: "/okteto/init",
			},
		},
	}

	c.Command = []string{
		"sh",
		"-c",
		fmt.Sprintf("(ls -A /okteto/init | grep -v lost+found || cp -Rf %s /okteto/init); touch /okteto/init/%s", source, dev.DevPath),
	}
	d.Spec.Template.Spec.InitContainers = []apiv1.Container{c}
}

func translateOktetoContainer(d *appsv1.Deployment, dev *model.Dev) {
	c := apiv1.Container{
		Name:            oktetoContainer,
		Image:           syncImageTag,
		ImagePullPolicy: apiv1.PullIfNotPresent,
		VolumeMounts: []apiv1.VolumeMount{
			apiv1.VolumeMount{
				Name:      oktetoSecretVolume,
				MountPath: "/var/syncthing/secret/",
			},
		},
		Ports: []apiv1.ContainerPort{
			apiv1.ContainerPort{
				ContainerPort: syncGUIPort,
			},
			apiv1.ContainerPort{
				ContainerPort: syncTCPPort,
			},
		},

		SecurityContext: &apiv1.SecurityContext{},
	}

	c.VolumeMounts = append(
		c.VolumeMounts,
		apiv1.VolumeMount{
			Name:      dev.GetVolumeName(),
			MountPath: oktetoMount,
		},
	)
	d.Spec.Template.Spec.Containers = append(d.Spec.Template.Spec.Containers, c)
}

func translateOktetoVolume(d *appsv1.Deployment, dev *model.Dev) {
	if d.Spec.Template.Spec.Volumes == nil {
		d.Spec.Template.Spec.Volumes = []apiv1.Volume{}
	}

	for _, v := range d.Spec.Template.Spec.Volumes {
		if v.Name == dev.GetVolumeName() {
			return
		}
	}
	v := apiv1.Volume{
		Name: dev.GetVolumeName(),
		VolumeSource: apiv1.VolumeSource{
			PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
				ClaimName: dev.GetVolumeName(),
				ReadOnly:  false,
			},
		},
	}
	d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes, v)
}

func translateOktetoSecretVolume(d *appsv1.Deployment, dev *model.Dev) {
	if d.Spec.Template.Spec.Volumes == nil {
		d.Spec.Template.Spec.Volumes = []apiv1.Volume{}
	}

	v := apiv1.Volume{
		Name: oktetoSecretVolume,
		VolumeSource: apiv1.VolumeSource{
			Secret: &apiv1.SecretVolumeSource{
				SecretName: dev.GetSecretName(),
			},
		},
	}

	d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes, v)
}
