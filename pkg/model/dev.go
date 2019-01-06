package model

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	yaml "gopkg.in/yaml.v2"
)

const (
	// CNDLabel is the label added to a dev deployment in k8
	CNDLabel = "cnd.okteto.com/deployment"

	// CNDDeploymentAnnotation is the original deployment manifest
	CNDDeploymentAnnotation = "cnd.okteto.com/deployment"

	// CNDDevAnnotation is the active cnd configuration
	CNDDevAnnotation = "cnd.okteto.com/dev"

	// CNDInitSyncContainerName is the name of the container initializing the shared volume
	CNDInitSyncContainerName = "cnd-init-syncthing"

	// CNDSyncContainerName is the name of the container running syncthing
	CNDSyncContainerName = "cnd-syncthing"

	// CNDSyncVolumeName is the name of synched volume
	CNDSyncVolumeName = "cnd-sync"
)

//Dev represents a cloud native development environment
type Dev struct {
	Swap    Swap              `yaml:"swap"`
	Mount   Mount             `yaml:"mount"`
	Scripts map[string]string `yaml:"scripts"`
}

//Swap represents the metadata for the container to be swapped
type Swap struct {
	Deployment Deployment `yaml:"deployment"`
}

//Deployment represents the container to be swapped
type Deployment struct {
	Name      string   `yaml:"name"`
	Container string   `yaml:"container,omitempty"`
	Image     string   `yaml:"image"`
	Command   []string `yaml:"command,omitempty"`
	Args      []string `yaml:"args,omitempty"`
}

//Mount represents how the local filesystem is mounted
type Mount struct {
	Source string `yaml:"source"`
	Target string `yaml:"target"`
}

//NewDev returns a new instance of dev with default values
func NewDev() *Dev {
	return &Dev{
		Swap: Swap{
			Deployment: Deployment{},
		},
		Mount: Mount{
			Source: ".",
			Target: "/app",
		},
		Scripts: make(map[string]string),
	}
}

func (dev *Dev) validate() error {
	file, err := os.Stat(dev.Mount.Source)
	if err != nil && os.IsNotExist(err) {
		return fmt.Errorf("Source mount folder %s does not exists", dev.Mount.Source)
	}
	if !file.Mode().IsDir() {
		return fmt.Errorf("Source mount folder is not a directory")
	}

	if dev.Swap.Deployment.Name == "" {
		return fmt.Errorf("Swap deployment name cannot be empty")
	}

	return nil
}

//ReadDev returns a Dev object from a given file
func ReadDev(devPath string) (*Dev, error) {
	b, err := ioutil.ReadFile(devPath)
	if err != nil {
		return nil, err
	}

	d, err := loadDev(b)
	if err != nil {
		return nil, err
	}

	if err := d.validate(); err != nil {
		return nil, err
	}

	d.fixPath(devPath)
	return d, nil
}

func loadDev(b []byte) (*Dev, error) {
	dev := Dev{
		Mount: Mount{
			Source: ".",
			Target: "/src",
		},
	}

	err := yaml.Unmarshal(b, &dev)
	if err != nil {
		return nil, err
	}

	if strings.HasPrefix(dev.Mount.Source, "~/") {
		home := os.Getenv("HOME")
		dev.Mount.Source = filepath.Join(home, dev.Mount.Source[2:])
	}

	return &dev, nil
}

func (dev *Dev) fixPath(originalPath string) {
	wd, _ := os.Getwd()

	if !filepath.IsAbs(dev.Mount.Source) {
		if filepath.IsAbs(originalPath) {
			dev.Mount.Source = path.Join(path.Dir(originalPath), dev.Mount.Source)
		} else {

			dev.Mount.Source = path.Join(wd, path.Dir(originalPath), dev.Mount.Source)
		}
	}
}
