package model

import (
	"fmt"
	"os"

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

var oktetoBaseDomain = os.Getenv("OKTETO_BASE_DOMAIN")

//Dev represents a development environment
type Dev struct {
	ID          string   `json:"id" yaml:"id"`
	Name        string   `json:"name" yaml:"name"`
	Image       string   `json:"image" yaml:"image"`
	Environment []EnvVar `json:"environment,omitempty" yaml:"environment,omitempty"`
	Command     []string `json:"command,omitempty" yaml:"command,omitempty"`
	WorkDir     string   `json:"workdir" yaml:"workdir"`
	DevPath     string
	Endpoints   []string `json:"endpoints,omitempty" yaml:"endpoints,omitempty"`
}

// EnvVar represents an environment value. When loaded, it will expand from the current env
type EnvVar struct {
	Name  string
	Value string
}

func read(bytes []byte) (*Dev, error) {
	dev := &Dev{
		WorkDir:     "/app",
		Image:       "okteto/desk:0.1.3",
		Environment: make([]EnvVar, 0),
		Command:     []string{"sh"},
	}
	if err := yaml.Unmarshal(bytes, dev); err != nil {
		return nil, err
	}

	return dev, nil
}

func (dev *Dev) validate() error {
	if dev.Name == "" {
		return fmt.Errorf("Name cannot be empty")
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

//GetEndpoint returns the dev environment endpoint
func (dev *Dev) GetEndpoint(s *Space) string {
	return fmt.Sprintf("%s-%s.%s", dev.Name, s.Name, oktetoBaseDomain)
}

//CertificateName returns the cretificate name for a dev environment
func (dev *Dev) CertificateName() string {
	return fmt.Sprintf("%s-letsencrypt", dev.Name)
}
