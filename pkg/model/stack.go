// Copyright 2020 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package model

import (
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"github.com/okteto/okteto/pkg/k8s/labels"
	yaml "gopkg.in/yaml.v2"
	apiv1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
)

var (
	errBadStackName = "must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character"
)

//Stack represents an okteto stack
type Stack struct {
	Services   map[string]*Service `yaml:"services,omitempty"`
}

//Service represents an okteto stack service
type Service struct {
	Deploy          *DeployInfo        `yaml:"deploy,omitempty"`
	Build           *BuildInfo         `yaml:"build,omitempty"`
	Replicas        int32              `yaml:"replicas"`
	Command         Command            `yaml:"command,omitempty"`
	Args            Args               `yaml:"args,omitempty"`
	Environment     []EnvVar           `yaml:"environment,omitempty"`
	EnvFiles        []string           `yaml:"env_file,omitempty"`
	CapAdd          []apiv1.Capability `yaml:"cap_add,omitempty"`
	CapDrop         []apiv1.Capability `yaml:"cap_drop,omitempty"`
	EnvFiles        []string           `yaml:"env_file,omitempty"`
	Environment     []EnvVar           `yaml:"enviroment,omitempty"`
	Ports           []Port             `yaml:"ports,omitempty"`
	Scale           int32              `yaml:"scale,omitempty"`
	StopGracePeriod time.Duration      `yaml:"stop_grace_period,omitempty"`
	Replicas  int32            `yaml:"replicas,omitempty"`
}

//ServiceResources represents an okteto stack service resources
type ServiceResources struct {
	CPU     Quantity        `json:"cpu,omitempty" yaml:"cpu,omitempty"`
	Memory  Quantity        `json:"memory,omitempty" yaml:"memory,omitempty"`
	Storage StorageResource `json:"storage,omitempty" yaml:"storage,omitempty"`
}

//StorageResource represents an okteto stack service storage resource
type StorageResource struct {
	Size  Quantity `json:"size,omitempty" yaml:"size,omitempty"`
	Class string   `json:"class,omitempty" yaml:"class,omitempty"`
}

//Quantity represents an okteto stack service storage resource
type Quantity struct {
	Value resource.Quantity
}

type DeployInfo struct {
	Replicas  int32                `yaml:"replicas,omitempty"`
	Resources ResourceRequirements `yaml:"resources,omitempty"`
}

type Port struct {
	Port     int32
	Public   bool
	Protocol apiv1.Protocol
}

//GetStack returns an okteto stack object from a given file
func GetStack(name, stackPath string) (*Stack, error) {
	b, err := ioutil.ReadFile(stackPath)
	if err != nil {
		return nil, err
	}

	s, err := ReadStack(b)
	if err != nil {
		return nil, err
	}

	if name != "" {
		s.Name = name
	}
	if s.Name == "" {
		s.Name, err = GetValidNameFromFolder(filepath.Dir(stackPath))
		if err != nil {
			return nil, err
		}
	}
	if err := s.validate(); err != nil {
		return nil, err
	}

	stackDir, err := filepath.Abs(filepath.Dir(stackPath))
	if err != nil {
		return nil, err
	}

	for name, svc := range s.Services {
		svc.extendPorts()
		svc.Public = svc.isPublic()
		if svc.Build == nil {
			continue
		}
		svc.Build.Context = loadAbsPath(stackDir, svc.Build.Context)
		svc.Build.Dockerfile = loadAbsPath(stackDir, svc.Build.Dockerfile)
		s.Services[name] = svc
	}
	return s, nil
}

//ReadStack reads an okteto stack
func ReadStack(bytes []byte) (*Stack, error) {
	s := &Stack{}
	if err := yaml.UnmarshalStrict(bytes, s); err != nil {
		if strings.HasPrefix(err.Error(), "yaml: unmarshal errors:") {
			var sb strings.Builder
			_, _ = sb.WriteString("Invalid stack manifest:\n")
			l := strings.Split(err.Error(), "\n")
			for i := 1; i < len(l); i++ {
				e := strings.TrimSuffix(l[i], "in type model.Stack")
				e = strings.TrimSpace(e)
				_, _ = sb.WriteString(fmt.Sprintf("    - %s\n", e))
			}

			_, _ = sb.WriteString("    See https://okteto.com/docs/reference/stacks for details")
			return nil, errors.New(sb.String())
		}

		msg := strings.Replace(err.Error(), "yaml: unmarshal errors:", "invalid stack manifest:", 1)
		msg = strings.TrimSuffix(msg, "in type model.Stack")
		return nil, errors.New(msg)
	}
	for i, svc := range s.Services {
		if svc.Build != nil {
			if svc.Build.Name != "" {
				svc.Build.Context = svc.Build.Name
				svc.Build.Name = ""
			}
			setBuildDefaults(svc.Build)
		}
		if svc.Replicas == 0 {
			svc.Replicas = 1
		}
		if svc.Resources.Storage.Size.Value.Cmp(resource.MustParse("0")) == 0 {
			svc.Resources.Storage.Size.Value = resource.MustParse("1Gi")
		}
		s.Services[i] = svc
	}
	return s, nil
}

func (s *Stack) validate() error {
	if err := validateStackName(s.Name); err != nil {
		return fmt.Errorf("Invalid stack name: %s", err)
	}
	if len(s.Services) == 0 {
		return fmt.Errorf("Invalid stack: 'services' cannot be empty")
	}

	for name, svc := range s.Services {
		if err := validateStackName(name); err != nil {
			return fmt.Errorf("Invalid service name '%s': %s", name, err)
		}
		if svc.Image == "" {
			return fmt.Errorf(fmt.Sprintf("Invalid service '%s': image cannot be empty", name))
		}
		for _, v := range svc.Volumes {
			if !strings.HasPrefix(v, "/") {
				return fmt.Errorf(fmt.Sprintf("Invalid volume '%s' in service '%s': must be an absolute path", v, name))
			}
			if strings.Contains(v, ":") {
				return fmt.Errorf(fmt.Sprintf("Invalid volume '%s' in service '%s': volume bind mounts are not supported", v, name))
			}
		}
	}

	return nil
}

func validateStackName(name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if ValidKubeNameRegex.MatchString(name) {
		return fmt.Errorf(errBadStackName)
	}
	if strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") {
		return fmt.Errorf(errBadStackName)
	}
	return nil
}

//UpdateNamespace updates the dev namespace
func (s *Stack) UpdateNamespace(namespace string) error {
	if namespace == "" {
		return nil
	}
	if s.Namespace != "" && s.Namespace != namespace {
		return fmt.Errorf("the namespace in the okteto stack manifest '%s' does not match the namespace '%s'", s.Namespace, namespace)
	}
	s.Namespace = namespace
	return nil
}

//GetLabelSelector returns the label selector for the stack name
func (s *Stack) GetLabelSelector() string {
	return fmt.Sprintf("%s=%s", labels.StackNameLabel, s.Name)
}

//GetLabelSelector returns the label selector for the stack name
func (s *Stack) GetConfigMapName() string {
	return fmt.Sprintf("okteto-%s", s.Name)
}

//SetLastBuiltAnnotationtamp sets the dev timestamp
func (svc *Service) SetLastBuiltAnnotationtamp() {
	if svc.Annotations == nil {
		svc.Annotations = map[string]string{}
	}
	svc.Annotations[labels.LastBuiltAnnotation] = time.Now().UTC().Format(labels.TimeFormat)
}

//extendPorts adds the ports that are in expose field to the port list.
func (svc *Service) extendPorts() bool {
	for _, port := range svc.Expose {
		if !svc.isAlreadyAdded(port) {
			svc.Ports = append(svc.Ports, Port{Port: port, Public: false, Protocol: apiv1.ProtocolTCP})
		}
	}
	return false
}

//isAlreadyAdded checks if a port is already on port list
func (svc *Service) isAlreadyAdded(p int32) bool {
	for _, port := range svc.Ports {
		if port.Port == p {
			return true
		}
	}
	return false
}

//isPublic sets the deploy resources and replicas of a service
func (svc *Service) isPublic() bool {
	for _, port := range svc.Ports {
		if port.Public {
			return true
		}
	}
	return false
}
