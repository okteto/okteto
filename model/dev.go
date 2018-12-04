package model

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"

	yaml "gopkg.in/yaml.v2"
)

// Dev represents a cloud native development environment
type Dev struct {
	Swap  swap  `yaml:"swap"`
	Mount mount `yaml:"mount"`
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
	if dev.Swap.Deployment.File == "" {
		return fmt.Errorf("Swap deployment file cannot be empty")
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
		Swap: swap{
			Deployment: deployment{
				Command: []string{"tail"},
				Args:    []string{"-f", "/dev/null"},
			},
		},
	}

	err := yaml.Unmarshal(b, &dev)
	if err != nil {
		return nil, err
	}

	if strings.HasPrefix(dev.Mount.Source, "~/") {
		usr, _ := user.Current()
		dir := usr.HomeDir
		dev.Mount.Source = filepath.Join(dir, dev.Mount.Source[2:])
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

	if !filepath.IsAbs(dev.Swap.Deployment.File) {
		if filepath.IsAbs(originalPath) {
			dev.Swap.Deployment.File = path.Join(path.Dir(originalPath), dev.Swap.Deployment.File)
		} else {

			dev.Swap.Deployment.File = path.Join(wd, path.Dir(originalPath), dev.Swap.Deployment.File)
		}
	}
}
