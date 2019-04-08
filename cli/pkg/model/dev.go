package model

import (
	"fmt"
	"io/ioutil"

	yaml "gopkg.in/yaml.v2"
	apiv1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
)

//Dev represents a cloud native development environment
type Dev struct {
	Swap         Swap                 `json:"swap,omitempty" yaml:"swap,omitempty"`
	Name         string               `json:"name" yaml:"name"`
	Container    string               `json:"container,omitempty" yaml:"container,omitempty"`
	Image        string               `json:"image" yaml:"image"`
	Environment  []EnvVar             `json:"environment,omitempty" yaml:"environment,omitempty"`
	Command      []string             `json:"command,omitempty" yaml:"command,omitempty"`
	Args         []string             `json:"args,omitempty" yaml:"args,omitempty"`
	EnableDocker bool                 `json:"enableDocker,omitempty" yaml:"enableDocker,omitempty"`
	Mount        *Mount               `json:"mount,omitempty" yaml:"mount,omitempty"`
	WorkDir      *Mount               `json:"workdir" yaml:"workdir"`
	Volumes      []Volume             `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	RunAsUser    *int64               `json:"runAsUser,omitempty" yaml:"runAsUser,omitempty"`
	Forward      []Forward            `json:"forward,omitempty" yaml:"forward,omitempty"`
	Resources    ResourceRequirements `json:"resources,omitempty" yaml:"resources,omitempty"`
	Scripts      map[string]string    `json:"scripts,omitempty" yaml:"scripts,omitempty"`
}

//Swap represents the metadata for the container to be swapped
type Swap struct {
	Deployment Deployment `json:"deployment" yaml:"deployment"`
}

//Deployment represents the container to be swapped
type Deployment struct {
	Name      string               `json:"name" yaml:"name"`
	Container string               `json:"container,omitempty" yaml:"container,omitempty"`
	Image     string               `json:"image" yaml:"image"`
	Command   []string             `json:"command,omitempty" yaml:"command,omitempty"`
	Args      []string             `json:"args,omitempty" yaml:"args,omitempty"`
	Resources ResourceRequirements `json:"resources,omitempty" yaml:"resources,omitempty"`
	RunAsUser *int64               `json:"runAsUser,omitempty" yaml:"runAsUser,omitempty"`
}

//Mount represents how the local filesystem is mounted
type Mount struct {
	SendOnly bool   `json:"sendonly,omitempty" yaml:"sendonly,omitempty"`
	Source   string `json:"source,omitempty" yaml:"source,omitempty"`
	Path     string `json:"path" yaml:"path,omitempty"`
	Target   string `json:"target,omitempty" yaml:"target,omitempty"` //TODO: decrecated
	Size     string `json:"size,omitempty" yaml:"size,omitempty"`
}

//Volume represents persistent volumes to be mounted in in a service.yml file
type Volume struct {
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	Path string `json:"path" yaml:"path,omitempty"`
	Size string `json:"size,omitempty" yaml:"size,omitempty"`
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
	return dev, nil
}

func read(bytes []byte) (*Dev, error) {
	dev := &Dev{
		Swap: Swap{
			Deployment: Deployment{
				Command: []string{"sh"},
				Resources: ResourceRequirements{
					Limits:   ResourceList{},
					Requests: ResourceList{},
				},
			},
		},
		Mount:       &Mount{},
		WorkDir:     &Mount{},
		Environment: make([]EnvVar, 0),
		Command:     make([]string, 0),
		Args:        make([]string, 0),
		Volumes:     make([]Volume, 0),
		Forward:     make([]Forward, 0),
		Resources: ResourceRequirements{
			Limits:   ResourceList{},
			Requests: ResourceList{},
		},
		Scripts: make(map[string]string),
	}
	if err := yaml.Unmarshal(bytes, dev); err != nil {
		return nil, err
	}
	dev.migrate()
	if err := dev.setDefaults(); err != nil {
		return nil, err
	}
	return dev, nil
}

func (dev *Dev) migrate() {
	if dev.Swap.Deployment.Name != "" {
		dev.Name = dev.Swap.Deployment.Name
		dev.Container = dev.Swap.Deployment.Container
		dev.Image = dev.Swap.Deployment.Image
		dev.Command = dev.Swap.Deployment.Command
		dev.Args = dev.Swap.Deployment.Args
		dev.Resources = dev.Swap.Deployment.Resources
		dev.RunAsUser = dev.Swap.Deployment.RunAsUser
	}
	if dev.Mount.Target != "" {
		dev.WorkDir.SendOnly = dev.Mount.SendOnly
		dev.WorkDir.Source = dev.Mount.Source
		dev.WorkDir.Path = dev.Mount.Target
		dev.WorkDir.Size = dev.Mount.Size
	}
}

func (dev *Dev) setDefaults() error {
	if len(dev.Command) == 0 {
		dev.Command = []string{"sh"}
	}
	if dev.WorkDir.Path == "" {
		dev.WorkDir.Path = "/okteto"
	}
	if dev.WorkDir.Size == "" {
		dev.WorkDir.Size = "10Gi"
	}
	if _, ok := dev.Resources.Limits[apiv1.ResourceCPU]; !ok {
		cpuLimit, _ := resource.ParseQuantity("1")
		dev.Resources.Limits[apiv1.ResourceCPU] = cpuLimit
	}
	if _, ok := dev.Resources.Limits[apiv1.ResourceMemory]; !ok {
		memoryLimit, _ := resource.ParseQuantity("2Gi")
		dev.Resources.Limits[apiv1.ResourceMemory] = memoryLimit
	}
	for i, v := range dev.Volumes {
		if v.Size == "" {
			dev.Volumes[i].Size = "10Gi"
		}
		if v.Name == "" {
			dev.Volumes[i].Name = fmt.Sprintf("volume-%d", i+1)
		}
	}

	return nil
}

func (dev *Dev) validate() error {
	if dev.Name == "" {
		return fmt.Errorf("Name cannot be empty")
	}

	return nil
}
