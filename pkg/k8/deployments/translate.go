package deployments

import (
	"encoding/json"

	"github.com/okteto/cnd/pkg/model"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
)

var (
	devReplicas                      int32 = 1
	devTerminationGracePeriodSeconds int64
)

func translateToDevModeDeployment(d *appsv1.Deployment, dev *model.Dev) error {

	d.Status = appsv1.DeploymentStatus{}
	manifest, err := json.Marshal(d)
	if err != nil {
		return err
	}
	setAnnotation(d.GetObjectMeta(), model.CNDDeploymentAnnotation, string(manifest))
	if err := setDevAsAnnotation(d, dev); err != nil {
		return err
	}
	setLabel(d.GetObjectMeta(), model.CNDLabel, d.Name)
	setLabel(d.Spec.Template.GetObjectMeta(), model.CNDLabel, d.Name)
	d.Spec.Template.Spec.TerminationGracePeriodSeconds = &devTerminationGracePeriodSeconds

	for i, c := range d.Spec.Template.Spec.Containers {
		if c.Name == dev.Swap.Deployment.Container || dev.Swap.Deployment.Container == "" {
			updateCndContainer(&d.Spec.Template.Spec.Containers[i], dev)
			break
		}
	}

	createInitSyncthingContainer(d, dev)
	createSyncthingContainer(d, dev)
	createSyncthingVolume(d, dev)

	if *(d.Spec.Replicas) != devReplicas {
		log.Info("cnd only supports running with 1 replica")
		d.Spec.Replicas = &devReplicas
	}
	return nil
}

func updateCndContainer(c *apiv1.Container, dev *model.Dev) {
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
		Name:      model.CNDSyncVolumeName,
		MountPath: dev.Mount.Target,
	}

	c.VolumeMounts = append(
		c.VolumeMounts,
		volumeMount,
	)

	c.Resources = apiv1.ResourceRequirements{}
}

func createInitSyncthingContainer(d *appsv1.Deployment, dev *model.Dev) {
	initSyncthingContainer := apiv1.Container{
		Name:  model.CNDInitSyncContainerName,
		Image: "okteto/init-syncthing:0.3.4",
		VolumeMounts: []apiv1.VolumeMount{
			apiv1.VolumeMount{
				Name:      model.CNDSyncVolumeName,
				MountPath: "/src",
			},
		},
	}

	if d.Spec.Template.Spec.InitContainers == nil {
		d.Spec.Template.Spec.InitContainers = []apiv1.Container{}
	}

	d.Spec.Template.Spec.InitContainers = append(d.Spec.Template.Spec.InitContainers, initSyncthingContainer)
}

func createSyncthingContainer(d *appsv1.Deployment, dev *model.Dev) {
	syncthingContainer := apiv1.Container{
		Name:  model.CNDSyncContainerName,
		Image: "okteto/syncthing:latest",
		VolumeMounts: []apiv1.VolumeMount{
			apiv1.VolumeMount{
				Name:      model.CNDSyncVolumeName,
				MountPath: "/var/cnd-sync",
			},
		},
		Ports: []apiv1.ContainerPort{
			apiv1.ContainerPort{
				ContainerPort: 8384,
			},
			apiv1.ContainerPort{
				ContainerPort: 22000,
			},
		},
	}

	d.Spec.Template.Spec.Containers = append(d.Spec.Template.Spec.Containers, syncthingContainer)
}

func createSyncthingVolume(d *appsv1.Deployment, dev *model.Dev) {
	if d.Spec.Template.Spec.Volumes == nil {
		d.Spec.Template.Spec.Volumes = []apiv1.Volume{}
	}

	syncVolume := apiv1.Volume{Name: model.CNDSyncVolumeName}

	d.Spec.Template.Spec.Volumes = append(
		d.Spec.Template.Spec.Volumes,
		syncVolume,
	)
}
