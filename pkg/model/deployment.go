package model

import (
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
)

type deployment struct {
	Name      string   `yaml:"name"`
	File      string   `yaml:"file"`
	Container string   `yaml:"container"`
	Image     string   `yaml:"image"`
	Command   []string `yaml:"command"`
	Args      []string `yaml:"args"`
}

const (
	// CNDRevisionAnnotation is the annotation added to a service to track the original deployment in k8
	CNDRevisionAnnotation = "deployment.okteto.com/parent"

	// CNDLabel is the label added to a dev deployment in k8
	CNDLabel = "deployment.okteto.com/cnd"

	// OldCNDLabel is the legacy label
	OldCNDLabel = "cnd"

	// RevisionAnnotation is the deployed revision
	RevisionAnnotation = "deployment.kubernetes.io/revision"

	// CNDSyncContainerName is the name of the container running syncthing
	CNDSyncContainerName = "cnd-syncthing"

	cndSyncVolumeName = "cnd-sync"
)

var (
	devReplicas int32 = 1
)

//TurnIntoDevDeployment modifies a  k8 deployment with the cloud native environment settings
func (dev *Dev) TurnIntoDevDeployment(d *appsv1.Deployment, parentRevision string) {

	labels := d.GetObjectMeta().GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}

	labels[CNDLabel] = d.Name

	annotations := d.GetObjectMeta().GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	annotations[CNDRevisionAnnotation] = parentRevision
	log.Debugf("dev deployment is based of revision %s", annotations[CNDRevisionAnnotation])

	d.GetObjectMeta().SetLabels(labels)
	d.GetObjectMeta().SetAnnotations(annotations)

	labels = d.Spec.Template.GetObjectMeta().GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}

	labels[CNDLabel] = d.Name

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
		log.Info("cnd only supports running with 1 replica")
		d.Spec.Replicas = &devReplicas
	}
}

func (dev *Dev) updateCndContainer(c *apiv1.Container) {
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
		Name:      cndSyncVolumeName,
		MountPath: dev.Mount.Target,
	}

	for i, v := range c.VolumeMounts {
		if v.Name == volumeMount.Name {
			c.VolumeMounts[i] = volumeMount
			return
		}
	}

	c.VolumeMounts = append(
		c.VolumeMounts,
		volumeMount,
	)
}

func (dev *Dev) createSyncthingContainer(d *appsv1.Deployment) {
	syncthingContainer := apiv1.Container{
		Name:  CNDSyncContainerName,
		Image: "okteto/syncthing:latest",
		VolumeMounts: []apiv1.VolumeMount{
			apiv1.VolumeMount{
				Name:      cndSyncVolumeName,
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

	for i, c := range d.Spec.Template.Spec.Containers {
		if c.Name == syncthingContainer.Name {
			d.Spec.Template.Spec.Containers[i] = syncthingContainer
			return
		}
	}

	d.Spec.Template.Spec.Containers = append(d.Spec.Template.Spec.Containers, syncthingContainer)
}

func (dev *Dev) createSyncthingVolume(d *appsv1.Deployment) {
	if d.Spec.Template.Spec.Volumes == nil {
		d.Spec.Template.Spec.Volumes = []apiv1.Volume{}
	}

	syncVolume := apiv1.Volume{Name: cndSyncVolumeName}

	for i, v := range d.Spec.Template.Spec.Volumes {
		if v.Name == syncVolume.Name {
			d.Spec.Template.Spec.Volumes[i] = syncVolume
			return
		}
	}

	d.Spec.Template.Spec.Volumes = append(
		d.Spec.Template.Spec.Volumes,
		syncVolume,
	)
}
