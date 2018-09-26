package model

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	yaml "gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	k8Yaml "k8s.io/apimachinery/pkg/util/yaml"
)

// Dev represents a cloud native development environment
type Dev struct {
	Name  string `yaml:"name"`
	Swap  swap   `yaml:"swap"`
	Mount mount  `yaml:"mount"`
}

type swap struct {
	Deployment deployment `yaml:"deployment"`
	Service    service    `yaml:"service"`
}

type deployment struct {
	File      string   `yaml:"file"`
	Container string   `yaml:"container"`
	Image     string   `yaml:"image"`
	Command   []string `yaml:"command"`
}

type service struct {
	File string `yaml:"file"`
}

type mount struct {
	Source string `yaml:"source"`
	Target string `yaml:"target"`
}

func (dev *Dev) validate() error {
	file, err := os.Stat(dev.Mount.Source)
	if err != nil && os.IsNotExist(err) {
		return fmt.Errorf("Source mount folder does not exists")
	}
	if !file.Mode().IsDir() {
		return fmt.Errorf("Source mount folder is not a directory")
	}
	if dev.Swap.Deployment.File == "" {
		return fmt.Errorf("Swap deployment file cannot be empty")
	}
	if dev.Swap.Deployment.Image == "" {
		return fmt.Errorf("Swap deployment image cannot be empty")
	}
	return nil
}

//ReadDev returns a Dev object from a given file
func ReadDev(devPath string) (*Dev, error) {
	readBytes, err := ioutil.ReadFile(devPath)
	if err != nil {
		return nil, err
	}
	dev := Dev{
		Mount: mount{
			Source: ".",
			Target: "/src",
		},
		Swap: swap{
			Deployment: deployment{
				Command: []string{"tail", "-f", "/dev/null"},
			},
		},
	}
	err = yaml.Unmarshal(readBytes, &dev)
	if err != nil {
		return nil, err
	}
	if err := dev.validate(); err != nil {
		return nil, err
	}
	if !filepath.IsAbs(dev.Mount.Source) {
		if filepath.IsAbs(devPath) {
			dev.Mount.Source = path.Join(devPath, dev.Mount.Source)
		} else {
			wd, _ := os.Getwd()
			dev.Mount.Source = path.Join(wd, devPath, dev.Mount.Source)
		}
	}
	return &dev, nil
}

//Deployment returns a k8 deployment for a cloud native environment
func (dev *Dev) Deployment() (*appsv1.Deployment, error) {
	cwd, _ := os.Getwd()
	file, err := os.Open(path.Join(cwd, dev.Swap.Deployment.File))
	if err != nil {
		return nil, err
	}
	dec := k8Yaml.NewYAMLOrJSONDecoder(file, 1000)
	var d appsv1.Deployment
	dec.Decode(&d)

	d.GetObjectMeta().SetName(dev.Name)
	labels := d.GetObjectMeta().GetLabels()
	if labels == nil {
		labels = map[string]string{"cnd": dev.Name}
	} else {
		labels["cnd"] = dev.Name
	}
	d.GetObjectMeta().SetLabels(labels)
	d.Spec.Selector.MatchLabels["cnd"] = dev.Name
	d.Spec.Template.GetObjectMeta().SetName(dev.Name)
	labels = d.Spec.Template.GetObjectMeta().GetLabels()
	if labels == nil {
		labels = map[string]string{"cnd": dev.Name}
	} else {
		labels["cnd"] = dev.Name
	}
	d.Spec.Template.GetObjectMeta().SetLabels(labels)

	a := apiv1.HostPathDirectory
	cndV := apiv1.Volume{
		Name: "git-volume",
		VolumeSource: apiv1.VolumeSource{
			HostPath: &apiv1.HostPathVolumeSource{
				Path: dev.Mount.Source,
				Type: &a,
			},
		},
	}
	d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes, cndV)
	for i, c := range d.Spec.Template.Spec.Containers {
		if c.Name == dev.Swap.Deployment.Container || dev.Swap.Deployment.Container == "" {
			d.Spec.Template.Spec.Containers[i].Image = dev.Swap.Deployment.Image
			d.Spec.Template.Spec.Containers[i].ImagePullPolicy = apiv1.PullIfNotPresent
			d.Spec.Template.Spec.Containers[i].Command = dev.Swap.Deployment.Command
			vM := apiv1.VolumeMount{
				Name:      "git-volume",
				MountPath: dev.Mount.Target,
			}
			d.Spec.Template.Spec.Containers[i].VolumeMounts = append(d.Spec.Template.Spec.Containers[i].VolumeMounts, vM)
			break
		}
	}

	return &d, nil
}

//Service returns a k8 service for a cloud native environment
func (dev *Dev) Service(translate bool) (*apiv1.Service, error) {
	cwd, _ := os.Getwd()
	file, err := os.Open(path.Join(cwd, dev.Swap.Service.File))
	if err != nil {
		return nil, err
	}
	dec := k8Yaml.NewYAMLOrJSONDecoder(file, 1000)
	var s apiv1.Service
	dec.Decode(&s)

	if !translate {
		return &s, nil
	}

	labels := s.GetObjectMeta().GetLabels()
	if labels == nil {
		labels = map[string]string{"cnd": dev.Name}
	} else {
		labels["cnd"] = dev.Name
	}
	s.GetObjectMeta().SetLabels(labels)
	s.Spec.Selector["cnd"] = dev.Name
	return &s, nil
}
