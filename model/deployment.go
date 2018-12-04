package model

import (
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
)

type deployment struct {
	File      string   `yaml:"file"`
	Container string   `yaml:"container"`
	Image     string   `yaml:"image"`
	Command   []string `yaml:"command"`
	Args      []string `yaml:"args"`
}

var (
	devReplicas int32 = 1
)

//TurnIntoDevDeployment modifies a  k8 deployment with the cloud native environment settings
func (dev *Dev) TurnIntoDevDeployment(d *appsv1.Deployment) {

	labels := d.GetObjectMeta().GetLabels()
	if labels == nil {
		labels = map[string]string{"cnd": d.Name}
	} else {
		labels["cnd"] = d.Name
	}

	d.GetObjectMeta().SetLabels(labels)

	labels = d.Spec.Template.GetObjectMeta().GetLabels()
	if labels == nil {
		labels = map[string]string{"cnd": d.Name}
	} else {
		labels["cnd"] = d.Name
	}

	d.Spec.Template.GetObjectMeta().SetLabels(labels)

	for i, c := range d.Spec.Template.Spec.Containers {
		if c.Name == dev.Swap.Deployment.Container || dev.Swap.Deployment.Container == "" {
			dev.updateCndContainer(&d.Spec.Template.Spec.Containers[i])
			break
		}
	}

	dev.createSyncthingContainer(d)
	dev.createSyncthingVolume(d)

	if *(d.Spec.Replicas) != devReplicas {
		log.Info("cnd only supports running with 1 replica in dev mode")
		d.Spec.Replicas = &devReplicas
	}
}

func (dev *Dev) updateCndContainer(c *apiv1.Container) {
	c.Image = dev.Swap.Deployment.Image
	c.ImagePullPolicy = apiv1.PullIfNotPresent
	c.Command = dev.Swap.Deployment.Command
	c.Args = dev.Swap.Deployment.Args
	c.WorkingDir = dev.Mount.Target
	if c.VolumeMounts == nil {
		c.VolumeMounts = []apiv1.VolumeMount{}
	}
	c.VolumeMounts = append(
		c.VolumeMounts,
		apiv1.VolumeMount{
			Name:      "cnd-sync",
			MountPath: dev.Mount.Target,
		},
	)

	c.ReadinessProbe = nil
	c.LivenessProbe = nil

}

func (dev *Dev) createSyncthingContainer(d *appsv1.Deployment) {
	d.Spec.Template.Spec.Containers = append(
		d.Spec.Template.Spec.Containers,
		apiv1.Container{
			Name:  "cnd-syncthing",
			Image: "okteto/syncthing:latest",
			VolumeMounts: []apiv1.VolumeMount{
				apiv1.VolumeMount{
					Name:      "cnd-sync",
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
		},
	)
}

func (dev *Dev) createSyncthingVolume(d *appsv1.Deployment) {
	if d.Spec.Template.Spec.Volumes == nil {
		d.Spec.Template.Spec.Volumes = []apiv1.Volume{}
	}

	d.Spec.Template.Spec.Volumes = append(
		d.Spec.Template.Spec.Volumes,
		apiv1.Volume{Name: "cnd-sync"},
	)
}
