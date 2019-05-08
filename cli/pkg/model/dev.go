package model

import (
	"fmt"
	"io/ioutil"

	yaml "gopkg.in/yaml.v2"
)

//Dev represents a cloud native development environment
type Dev struct {
	Name        string   `json:"name" yaml:"name"`
	Space       string   `json:"space" yaml:"space"`
	Image       string   `json:"image" yaml:"image"`
	Environment []EnvVar `json:"environment,omitempty" yaml:"environment,omitempty"`
	Command     []string `json:"command,omitempty" yaml:"command,omitempty"`
	Volumes     []string `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	WorkDir     string   `json:"workdir" yaml:"workdir"`
}

// EnvVar represents an environment value. When loaded, it will expand from the current env
type EnvVar struct {
	Name  string
	Value string
}

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
		Environment: make([]EnvVar, 0),
		Command:     make([]string, 0),
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
	if len(dev.Volumes) > 2 {
		return fmt.Errorf("The maximum number of volumes is 2")
	}

	return nil
}
