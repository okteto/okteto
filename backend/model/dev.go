package model

import (
	"fmt"

	yaml "gopkg.in/yaml.v2"
)

const (
	oktetoVolumeTemplate = "okteto-%s"
	oktetoSecretTemplate = "okteto-%s"
)

var supportedServices = map[string]bool{
	"redis":    true,
	"mongodb":  true,
	"mysql":    true,
	"postgres": true,
}

//Dev represents a development environment
type Dev struct {
	Name        string   `json:"name" yaml:"name"`
	Image       string   `json:"image" yaml:"image"`
	Environment []EnvVar `json:"environment,omitempty" yaml:"environment,omitempty"`
	Command     []string `json:"command,omitempty" yaml:"command,omitempty"`
	WorkDir     *Mount   `json:"workdir" yaml:"workdir"`
	Services    []string `json:"services,omitempty" yaml:"services,omitempty"`
}

//Mount represents how the local filesystem is mounted
type Mount struct {
	Source string `json:"source,omitempty" yaml:"source,omitempty"`
	Path   string `json:"path" yaml:"path,omitempty"`
	Size   string `json:"size,omitempty" yaml:"size,omitempty"`
}

// EnvVar represents an environment value. When loaded, it will expand from the current env
type EnvVar struct {
	Name  string
	Value string
}

func read(bytes []byte) (*Dev, error) {
	dev := &Dev{
		WorkDir:     &Mount{},
		Image:       "okteto/desk:0.1.2",
		Environment: make([]EnvVar, 0),
		Command:     make([]string, 0),
		Services:    make([]string, 0),
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
	if dev.WorkDir.Path == "" {
		dev.WorkDir.Path = "/okteto"
	}
	if dev.WorkDir.Size == "" {
		dev.WorkDir.Size = "10Gi"
	}
	return nil
}

func (dev *Dev) validate() error {
	if dev.Name == "" {
		return fmt.Errorf("Name cannot be empty")
	}

	for _, service := range dev.Services {
		if _, ok := supportedServices[service]; !ok {
			return fmt.Errorf("Unsupported service '%s'", service)
		}
	}

	return nil
}

//GetVolumeName returns the okteto volume name for a given dev environment
func (dev *Dev) GetVolumeName() string {
	return fmt.Sprintf(oktetoVolumeTemplate, dev.Name)
}

//GetSecretName returns the okteto secret name for a given dev environment
func (dev *Dev) GetSecretName() string {
	return fmt.Sprintf(oktetoSecretTemplate, dev.Name)
}
