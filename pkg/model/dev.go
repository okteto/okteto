package model

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	yaml "gopkg.in/yaml.v2"
	apiv1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
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
	Volumes     []string             `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	Forward     []Forward            `json:"forward,omitempty" yaml:"forward,omitempty"`
	Resources   ResourceRequirements `json:"resources,omitempty" yaml:"resources,omitempty"`
	DevPath     string               `json:"-" yaml:"-"`
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
		Resources: ResourceRequirements{
			Limits:   ResourceList{},
			Requests: ResourceList{},
		},
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
	if dev.WorkDir == "" {
		dev.WorkDir = "/okteto"
	}
	return nil
}

func (dev *Dev) validate() error {
	if dev.Name == "" {
		return fmt.Errorf("Name cannot be empty")
	}
	return nil
}
