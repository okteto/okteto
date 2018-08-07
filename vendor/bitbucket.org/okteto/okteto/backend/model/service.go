package model

import (
	"fmt"
	"regexp"
)

var isAlphaNumeric = regexp.MustCompile(`^[a-zA-Z0-9][-a-zA-Z0-9]*$`).MatchString

// ServiceStatus is the current state of a service
type ServiceStatus string

const (

	//CreatingService is the state when the service is being created
	CreatingService ServiceStatus = "creating"

	//CreatedService is the state when the service is created
	CreatedService ServiceStatus = "created"

	//DeployingService is the state when the service is being deployed
	DeployingService ServiceStatus = "deploying"

	//DeployedService is the state when the service is deployed
	DeployedService ServiceStatus = "deployed"

	//DestroyingService is the state when the service is being destroyed
	DestroyingService ServiceStatus = "destroying"

	//DestroyedService is the state when the service is destroyed its Service
	DestroyedService ServiceStatus = "destroyed"

	//FailedService is the state when the service failed to change state
	FailedService ServiceStatus = "failed"

	//Unknown is the state when the service is in an unknown state
	Unknown ServiceStatus = "unknown"
)

// ServiceLinks contains links to the activities and to itself
type ServiceLinks struct {
	Activities string `json:"activities,omitempty"`
	Self       string `json:"self,omitempty"`
}

//Service represents a service.yml file
type Service struct {
	Model
	Name      string        `json:"name" yaml:"name,omitempty"`
	Status    ServiceStatus `json:"state" gorm:"index"  yaml:"-"`
	ProjectID string        `json:"project,omitempty" yaml:"-" gorm:"index"`
	DNS       string        `json:"-" gorm:"dns" yaml:"-"`
	Manifest  string        `json:"manifest,omitempty" gorm:"manifest"  yaml:"-"`

	// YAML content
	Replicas     int                   `json:"replicas,omitempty" yaml:"replicas,omitempty" gorm:"-"`
	Stateful     bool                  `json:"stateful,omitempty" yaml:"stateful,omitempty" gorm:"-"`
	Public       bool                  `json:"public,omitempty" yaml:"public,omitempty" gorm:"-"`
	InstanceType string                `json:"instance_type,omitempty" yaml:"instance_type,omitempty" gorm:"-"`
	Containers   map[string]*Container `json:"containers,omitempty" yaml:"containers,omitempty" gorm:"-"`

	// Linked resources
	Activities []Activity `json:"activities,omitempty" yaml:"-"`

	// generated links
	Logs      string       `json:"logs,omitempty" gorm:"-"  yaml:"-"`
	Endpoints []string     `json:"endpoints,omitempty" gorm:"-"  yaml:"-"`
	Links     ServiceLinks `json:"links,omitempty" gorm:"-"  yaml:"-"`
}

//Container represents a container in a service.yml file
type Container struct {
	Image       string     `json:"image,omitempty" yaml:"image,omitempty"`
	Command     string     `json:"command,omitempty" yaml:"command,omitempty"`
	Ports       []*Port    `json:"ports,omitempty" yaml:"ports,omitempty"`
	Environment []*EnvVar  `json:"environment,omitempty" yaml:"environment,omitempty"`
	Resources   *Resources `json:"resources,omitempty" yaml:"resources,omitempty"`
}

//Port represents a container port
type Port struct {
	Certificate      string `json:"certificate,omitempty" yaml:"certificate,omitempty"`
	InstanceProtocol string `json:"instance_protocol,omitempty" yaml:"instance_protocol,omitempty"`
	InstancePort     string `json:"instance_port,omitempty" yaml:"instance_port,omitempty"`
	Protocol         string `json:"protocol,omitempty" yaml:"protocol,omitempty"`
	Port             string `json:"port,omitempty" yaml:"port,omitempty"`
}

//EnvVar represents a container envvar
type EnvVar struct {
	Name  string `json:"name,omitempty" yaml:"name,omitempty"`
	Value string `json:"value,omitempty" yaml:"value,omitempty"`
}

//Resources represents the container resources
type Resources struct {
	Limits   *Resource `json:"limits,omitempty" yaml:"limits,omitempty"`
	Requests *Resource `json:"requests,omitempty" yaml:"requests,omitempty"`
}

//Resource represents a container resource
type Resource struct {
	Memory string `json:"memory,omitempty" yaml:"memory,omitempty"`
	CPU    string `json:"cpu,omitempty" yaml:"cpu,omitempty"`
}

//UnmarshalYAML sets the default value of replica to 1
func (s *Service) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type rawService Service
	raw := rawService{}
	raw.Replicas = 1
	if err := unmarshal(&raw); err != nil {
		return err
	}

	*s = Service(raw)
	return nil
}

//Validate returns an error for invalid service.yml files
func (s *Service) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("'service.name' is mandatory")
	}
	if !isAlphaNumeric(s.Name) {
		return fmt.Errorf("'service.name' only allows alphanumeric characters or dashes")
	}
	if s.Stateful && s.Replicas != 1 {
		return fmt.Errorf("Stateful services can only have one replica")
	}
	return nil
}

//GetInstanceType returns the service size taking into account default values
func (s *Service) GetInstanceType(e *Environment) string {
	if s.InstanceType != "" {
		return s.InstanceType
	}
	if e.Provider != nil && e.Provider.InstanceType != "" {
		return e.Provider.InstanceType
	}
	return "t2.small"
}

//GetPorts returns the list of ports of a service
func (s *Service) GetPorts() []*Port {
	result := []*Port{}
	for _, container := range s.Containers {
		for _, port := range container.Ports {
			result = append(result, port)
		}
	}
	return result
}

// CalculateStatus calculates the Service state of d based on a
func (s *Service) CalculateStatus(a *Activity) ServiceStatus {
	if a == nil {
		return FailedService
	}

	if a.Status == Failed {
		return FailedService
	}

	switch a.Type {
	case Created:
		if a.Status == InProgress {
			return CreatingService
		}
		return CreatedService

	case Deployed:
		if a.Status == InProgress {
			return DeployingService
		}
		return DeployedService
	case Destroyed:
		if a.Status == InProgress {
			return DestroyingService
		}
		return DestroyedService
	default:
		return Unknown
	}
}

// CanDeploy returns true if d is in a state that allows a deploy operation
func (s *Service) CanDeploy() bool {
	if s.Status == CreatingService || s.Status == DeployingService || s.Status == DestroyingService ||
		s.Status == DestroyedService {
		return false
	}

	return true
}

// CanDestroy returns true if d is in a state that allows a destroy operation
func (s *Service) CanDestroy() bool {
	if s.Status == CreatingService || s.Status == DeployingService || s.Status == DestroyingService ||
		s.Status == DestroyedService {
		return false
	}

	return true
}

// IsDestroyed returns true if d is in a state of destruction
func (s *Service) IsDestroyed() bool {
	if s.Status == DestroyingService || s.Status == DestroyedService {
		return true
	}

	return false
}
