package model

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v2"
	apiResource "k8s.io/apimachinery/pkg/api/resource"
)

// Dev represents a cloud native development environment
type Dev struct {
	Swap    swap              `yaml:"swap"`
	Mount   mount             `yaml:"mount"`
	Scripts map[string]string `yaml:"scripts"`
}

type swap struct {
	Deployment deployment `yaml:"deployment"`
}

type mount struct {
	Source string `yaml:"source"`
	Target string `yaml:"target"`
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
		if dev.Swap.Deployment.File != "" {
			// for legacy deployments
			return fmt.Errorf("Swap deployment name cannot be empty")
		}

	}

	if err := validateQuantity(dev.Swap.Deployment.Resources.Limits.CPU, "dev.swap.deployment.resources.limits.cpu"); err != nil {
		return err
	}

	if err := validateQuantity(dev.Swap.Deployment.Resources.Limits.Memory, "dev.swap.deployment.resources.limits.memory"); err != nil {
		return err
	}

	if err := validateQuantity(dev.Swap.Deployment.Resources.Requests.CPU, "dev.swap.deployment.resources.requests.cpu"); err != nil {
		return err
	}

	if err := validateQuantity(dev.Swap.Deployment.Resources.Requests.Memory, "dev.swap.deployment.resources.requests.memory"); err != nil {
		return err
	}

	return nil
}

func validateQuantity(quantity, tagName string) error {
	if quantity == "" {
		return nil
	}

	_, err := apiResource.ParseQuantity(quantity)
	if err != nil {
		log.Errorf("invalid quantity value for %s: %s", tagName, err)
		return fmt.Errorf("%s is not a valid value for %s", quantity, tagName)
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
		Mount: mount{
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
