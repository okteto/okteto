package model

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	yaml "gopkg.in/yaml.v2"
	apiv1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
)

const (
	oktetoVolumeTemplate      = "okteto-%s"
	oktetoStatefulSetTemplate = "okteto-%s"
	oktetoVolumeDataTemplate  = "okteto-%d-%s"
)

//Dev represents a cloud native development environment
type Dev struct {
	Name        string               `json:"name" yaml:"name"`
	Namespace   string               `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Container   string               `json:"container,omitempty" yaml:"container,omitempty"`
	Image       string               `json:"image,omitempty" yaml:"image,omitempty"`
	Environment []EnvVar             `json:"environment,omitempty" yaml:"environment,omitempty"`
	Command     []string             `json:"command,omitempty" yaml:"command,omitempty"`
	WorkDir     string               `json:"workdir" yaml:"workdir"`
	MountPath   string               `json:"mountpath,omitempty" yaml:"mountpath,omitempty"`
	SubPath     string               `json:"subpath,omitempty" yaml:"subpath,omitempty"`
	Volumes     []string             `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	Forward     []Forward            `json:"forward,omitempty" yaml:"forward,omitempty"`
	Resources   ResourceRequirements `json:"resources,omitempty" yaml:"resources,omitempty"`
	DevPath     string               `json:"-" yaml:"-"`
	Services    []Dev                `json:"services,omitempty" yaml:"services,omitempty"`
}

// EnvVar represents an environment value. When loaded, it will expand from the current env
type EnvVar struct {
	Name  string
	Value string
}

// Forward represents a port forwarding definition
type Forward struct {
	Local  int
	Remote int
}

// ResourceRequirements describes the compute resource requirements.
type ResourceRequirements struct {
	Limits   ResourceList
	Requests ResourceList
}

// ResourceList is a set of (resource name, quantity) pairs.
type ResourceList map[apiv1.ResourceName]resource.Quantity

//Get returns a Dev object from a given file
func Get(devPath string) (*Dev, error) {
	b, err := ioutil.ReadFile(devPath)
	if err != nil {
		return nil, err
	}

	dev, err := read(b)
	if err != nil {
		return nil, err
	}

	if err := dev.validate(); err != nil {
		return nil, err
	}
	dev.DevPath = filepath.Base(devPath)

	return dev, nil
}

func read(bytes []byte) (*Dev, error) {
	dev := &Dev{
		Environment: make([]EnvVar, 0),
		Command:     make([]string, 0),
		Forward:     make([]Forward, 0),
		Volumes:     make([]string, 0),
		Resources: ResourceRequirements{
			Limits:   ResourceList{},
			Requests: ResourceList{},
		},
		Services: make([]Dev, 0),
	}
	if err := yaml.Unmarshal(bytes, dev); err != nil {
		return nil, err
	}
	if err := dev.setDefaults(); err != nil {
		return nil, err
	}
	return dev, nil
}

func (dev *Dev) setDefaults() error {
	if len(dev.Command) == 0 {
		dev.Command = []string{"sh"}
	}
	if dev.MountPath == "" && dev.WorkDir == "" {
		dev.MountPath = "/okteto"
		dev.WorkDir = "/okteto"
	}
	if dev.WorkDir != "" && dev.MountPath == "" {
		dev.MountPath = dev.WorkDir
	}
	for i, s := range dev.Services {
		if s.MountPath == "" && s.WorkDir == "" {
			dev.Services[i].MountPath = "/okteto"
			dev.Services[i].WorkDir = "/okteto"
		}
		if s.WorkDir != "" && s.MountPath == "" {
			dev.Services[i].MountPath = s.WorkDir
		}
		dev.Services[i].Forward = make([]Forward, 0)
		dev.Services[i].Volumes = make([]string, 0)
		dev.Services[i].Services = make([]Dev, 0)
	}
	return nil
}

func (dev *Dev) validate() error {
	if dev.Name == "" {
		return fmt.Errorf("Name cannot be empty")
	}
	return nil
}

//GetSyncStatefulSetName returns the syncthing statefulset name for a given dev environment
func (dev *Dev) GetSyncStatefulSetName() string {
	n := fmt.Sprintf(oktetoStatefulSetTemplate, dev.Name)
	if len(n) > 63 {
		n = n[0:63]
	}

	return n
}

//GetSyncVolumeName returns the syncthing volume name for a given dev environment
func (dev *Dev) GetSyncVolumeName() string {
	n := fmt.Sprintf(oktetoVolumeTemplate, dev.Name)
	if len(n) > 63 {
		n = n[0:63]
	}

	return n
}

//GetDataVolumeName returns the data volume name for a given dev environment
func (dev *Dev) GetDataVolumeName(i int) string {
	n := fmt.Sprintf(oktetoVolumeDataTemplate, i, dev.Name)
	if len(n) > 63 {
		n = n[0:63]
	}

	return n
}
