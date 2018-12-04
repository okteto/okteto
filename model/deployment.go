package model

import (
	"os"

	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	k8Yaml "k8s.io/apimachinery/pkg/util/yaml"
)

type deployment struct {
	File      string   `yaml:"file"`
	Container string   `yaml:"container"`
	Image     string   `yaml:"image"`
	Command   []string `yaml:"command"`
	Args      []string `yaml:"args"`
}

//Deployment returns a k8 deployment
func (dev *Dev) Deployment() (*appsv1.Deployment, error) {
	return dev.loadDeployment()
}

//DevDeployment returns a k8 deployment modified with  a cloud native environment
func (dev *Dev) DevDeployment() (*appsv1.Deployment, error) {
	d, err := dev.loadDeployment()
	if err != nil {
		return nil, err
	}

	labels := d.GetObjectMeta().GetLabels()
	if labels == nil {
		labels = map[string]string{"cnd": dev.Name}
	} else {
		labels["cnd"] = dev.Name
	}
	d.GetObjectMeta().SetLabels(labels)

	labels = d.Spec.Template.GetObjectMeta().GetLabels()
	if labels == nil {
		labels = map[string]string{"cnd": dev.Name}
	} else {
		labels["cnd"] = dev.Name
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

	return d, nil
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

func (dev *Dev) loadDeployment() (*appsv1.Deployment, error) {
	log.Debugf("loading deployment definition from %s", dev.Swap.Deployment.File)
	file, err := os.Open(dev.Swap.Deployment.File)
	if err != nil {
		return nil, err
	}

	dec := k8Yaml.NewYAMLOrJSONDecoder(file, 1000)
	var d appsv1.Deployment
	err = dec.Decode(&d)
	return &d, err
}
