package model

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	yaml "gopkg.in/yaml.v2"
)

const (
	// CNDLabel is the label added to a dev deployment in k8
	CNDLabel = "cnd.okteto.com/deployment"

	// CNDDeploymentAnnotation is the original deployment manifest
	CNDDeploymentAnnotation = "cnd.okteto.com/deployment"

	// CNDDevListAnnotation is the list of cnd manifest annotations
	CNDDevListAnnotation = "cnd.okteto.com/cnd"

	// CNDSyncContainer is the name of the container running syncthing
	CNDSyncContainer = "cnd-sync"

	// CNDSyncSecretVolume is the name of the volume mounting the secret
	CNDSyncSecretVolume = "cnd-sync-secret"

	cndDevAnnotationTemplate     = "cnd.okteto.com/dev-%s"
	cndInitSyncContainerTemplate = "cnd-init-%s"
	cndSyncVolumeTemplate        = "cnd-data-%s"
	cndSyncMountTemplate         = "/var/cnd-sync/%s"
	cndSyncSecretTemplate        = "cnd-secret-%s"
)

//Dev represents a cloud native development environment
type Dev struct {
	Swap        Swap              `json:"swap" yaml:"swap"`
	Mount       Mount             `json:"mount" yaml:"mount"`
	Scripts     map[string]string `json:"scripts" yaml:"scripts"`
	Environment []EnvVar          `json:"environment,omitempty" yaml:"environment,omitempty"`
	Forward     []Forward         `json:"forward,omitempty" yaml:"forward,omitempty"`
}

//Swap represents the metadata for the container to be swapped
type Swap struct {
	Deployment Deployment `json:"deployment" yaml:"deployment"`
}

//Deployment represents the container to be swapped
type Deployment struct {
	Name      string   `json:"name" yaml:"name"`
	Container string   `json:"container,omitempty" yaml:"container,omitempty"`
	Image     string   `json:"image" yaml:"image"`
	Command   []string `json:"command,omitempty" yaml:"command,omitempty"`
	Args      []string `json:"args,omitempty" yaml:"args,omitempty"`
}

//Mount represents how the local filesystem is mounted
type Mount struct {
	Source string `json:"source" yaml:"source"`
	Target string `json:"target" yaml:"target"`
}

// EnvVar represents an environment value. When loaded, it will expand from the current env
type EnvVar struct {
	Name  string
	Value string
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (e *EnvVar) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw string
	err := unmarshal(&raw)
	if err != nil {
		return err
	}

	parts := strings.SplitN(raw, "=", 2)
	e.Name = parts[0]
	if len(parts) == 2 {
		if strings.HasPrefix(parts[1], "$") {
			e.Value = os.ExpandEnv(parts[1])
			return nil
		}

		e.Value = parts[1]
		return nil
	}

	val := os.ExpandEnv(parts[0])
	if val != parts[0] {
		e.Value = val
	}

	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (e *EnvVar) MarshalYAML() (interface{}, error) {
	return e.Name + "=" + e.Value, nil
}

// Forward represents a port forwarding definition
type Forward struct {
	Local  int
	Remote int
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (f *Forward) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw string
	err := unmarshal(&raw)
	if err != nil {
		return err
	}

	parts := strings.SplitN(raw, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("Wrong port-forward syntax '%s', must be of the form 'localPort:RemotePort'", raw)
	}
	localPort, err := strconv.Atoi(parts[0])
	if err != nil {
		return fmt.Errorf("Cannot convert remote port '%s' in port-forward '%s'", parts[0], raw)
	}
	remotePort, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("Cannot convert remote port '%s' in port-forward '%s'", parts[1], raw)
	}
	f.Local = localPort
	f.Remote = remotePort
	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (f *Forward) MarshalYAML() (interface{}, error) {
	return fmt.Sprintf("%d:%d", f.Local, f.Remote), nil
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
		Scripts:     make(map[string]string),
		Environment: make([]EnvVar, 0),
		Forward:     make([]Forward, 0),
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

	d, err := LoadDev(b)
	if err != nil {
		return nil, err
	}

	if err := d.validate(); err != nil {
		return nil, err
	}

	d.fixPath(devPath)
	return d, nil
}

// LoadDev loads the dev object from the array, plus defaults.
func LoadDev(b []byte) (*Dev, error) {
	dev := NewDev()

	err := yaml.Unmarshal(b, dev)
	if err != nil {
		return nil, err
	}

	if strings.HasPrefix(dev.Mount.Source, "~/") {
		home := os.Getenv("HOME")
		dev.Mount.Source = filepath.Join(home, dev.Mount.Source[2:])
	}

	return dev, nil
}

func (dev *Dev) fixPath(originalPath string) {
	wd, _ := os.Getwd()

	if !filepath.IsAbs(dev.Mount.Source) {
		if filepath.IsAbs(originalPath) {
			dev.Mount.Source = filepath.Join(filepath.Dir(originalPath), dev.Mount.Source)
		} else {

			dev.Mount.Source = filepath.Join(wd, filepath.Dir(originalPath), dev.Mount.Source)
		}
	}
}

// GetCNDInitSyncContainer returns the CND init sync container name for a given container
func (dev *Dev) GetCNDInitSyncContainer() string {
	return fmt.Sprintf(cndInitSyncContainerTemplate, dev.Swap.Deployment.Container)
}

// GetCNDSyncVolume returns the CND sync volume name for a given container
func (dev *Dev) GetCNDSyncVolume() string {
	return fmt.Sprintf(cndSyncVolumeTemplate, dev.Swap.Deployment.Container)
}

// GetCNDSyncMount returns the CND sync mount for a given container
func (dev *Dev) GetCNDSyncMount() string {
	return fmt.Sprintf(cndSyncMountTemplate, dev.Swap.Deployment.Container)
}

// GetCNDSyncSecret returns the CND sync secret for a given deployment
func GetCNDSyncSecret(deployment string) string {
	return fmt.Sprintf(cndSyncSecretTemplate, deployment)
}
