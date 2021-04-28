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
	Manifest            []byte                 `yaml:"-"`
	Warnings            []string               `yaml:"-"`
	VolumeMountWarnings []string               `yaml:"-"`
	IsCompose           bool                   `yaml:"-"`
	Name                string                 `yaml:"name"`
	Volumes             map[string]*VolumeSpec `yaml:"volumes,omitempty"`
	Namespace           string                 `yaml:"namespace,omitempty"`
	Services            map[string]*Service    `yaml:"services,omitempty"`
	Endpoints           map[string]Endpoint    `yaml:"endpoints,omitempty"`
}

//Service represents an okteto stack service
type Service struct {
	Build      *BuildInfo         `yaml:"build,omitempty"`
	CapAdd     []apiv1.Capability `yaml:"cap_add,omitempty"`
	CapDrop    []apiv1.Capability `yaml:"cap_drop,omitempty"`
	Entrypoint Entrypoint         `yaml:"entrypoint,omitempty"`
	Command    Command            `yaml:"command,omitempty"`
	EnvFiles   EnvFiles           `yaml:"env_file,omitempty"`

	Environment     Environment   `yaml:"environment,omitempty"`
	Expose          []int32       `yaml:"expose,omitempty"`
	Image           string        `yaml:"image,omitempty"`
	Labels          Labels        `json:"labels,omitempty" yaml:"labels,omitempty"`
	Annotations     Annotations   `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Ports           []Port        `yaml:"ports,omitempty"`
	StopGracePeriod int64         `yaml:"stop_grace_period,omitempty"`
	Volumes         []StackVolume `yaml:"volumes,omitempty"`
	WorkingDir      string        `yaml:"working_dir,omitempty"`

	Public    bool            `yaml:"public,omitempty"`
	Replicas  int32           `yaml:"replicas,omitempty"`
	Resources *StackResources `yaml:"resources,omitempty"`
}

type StackVolume struct {
	LocalPath  string
	RemotePath string
}

type VolumeSpec struct {
	Name        string            `yaml:"name,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty"`
	Storage     StorageResource   `json:"storage,omitempty" yaml:"storage,omitempty"`
}
type Envs struct {
	List Environment
}

//StackResources represents an okteto stack resources
type StackResources struct {
	Limits   ServiceResources `json:"limits,omitempty" yaml:"limits,omitempty"`
	Requests ServiceResources `json:"requests,omitempty" yaml:"requests,omitempty"`
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

type Port struct {
	Port     int32
	Protocol apiv1.Protocol
}

//Endpoints represents an okteto stack ingress
type Endpoint struct {
	Labels      Labels         `json:"labels,omitempty" yaml:"labels,omitempty"`
	Annotations Annotations    `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Rules       []EndpointRule `yaml:"rules,omitempty"`
}

// EndpointRule represents an okteto ingress rule
type EndpointRule struct {
	Path    string `yaml:"path,omitempty"`
	Service string `yaml:"service,omitempty"`
	Port    int32  `yaml:"port,omitempty"`
}

//GetStack returns an okteto stack object from a given file
func GetStack(name, stackPath string, isCompose bool) (*Stack, error) {
	b, err := ioutil.ReadFile(stackPath)
	if err != nil {
		return nil, err
	}

	s, err := ReadStack(b, isCompose)
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

	for _, svc := range s.Services {
		svc.extendPorts()
		svc.IgnoreSyncVolumes(s)
		if svc.Build == nil {
			continue
		}
		svc.Build.Context = loadAbsPath(stackDir, svc.Build.Context)
		svc.Build.Dockerfile = loadAbsPath(stackDir, svc.Build.Dockerfile)

	}
	return s, nil
}

//ReadStack reads an okteto stack
func ReadStack(bytes []byte, isCompose bool) (*Stack, error) {
	s := &Stack{
		Manifest:  bytes,
		IsCompose: isCompose,
	}

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
	for _, svc := range s.Services {
		if svc.Build != nil {
			if svc.Build.Name != "" {
				svc.Build.Context = svc.Build.Name
				svc.Build.Name = ""
			}
			setBuildDefaults(svc.Build)
		}
		if svc.Resources.Requests.Storage.Size.Value.Cmp(resource.MustParse("0")) == 0 {
			svc.Resources.Requests.Storage.Size.Value = resource.MustParse("1Gi")
		}

		if svc.Replicas == 0 {
			svc.Replicas = 1
		}

		if len(svc.Expose) > 0 && len(svc.Ports) == 0 {
			svc.Public = false
		}

		if len(svc.Expose) > 0 {
			svc.extendPorts()
		}

	}
	for _, volume := range s.Volumes {
		if volume.Storage.Size.Value.Cmp(resource.MustParse("0")) == 0 {
			volume.Storage.Size.Value = resource.MustParse("1Gi")
		}
	}
	return s, nil
}

func (svc *Service) IgnoreSyncVolumes(s *Stack) {
	notIgnoredVolumes := make([]StackVolume, 0)
	for _, volume := range svc.Volumes {
		if volume.LocalPath == "" || isInVolumesTopLevelSection(volume.LocalPath, s) {
			notIgnoredVolumes = append(notIgnoredVolumes, volume)
		}
	}
	svc.Volumes = notIgnoredVolumes
}

func (s *Stack) validate() error {
	if err := validateStackName(s.Name); err != nil {
		return fmt.Errorf("Invalid stack name: %s", err)
	}
	if len(s.Services) == 0 {
		return fmt.Errorf("Invalid stack: 'services' cannot be empty")
	}

	for endpointName, endpoint := range s.Endpoints {
		for _, endpointRule := range endpoint.Rules {
			if service, ok := s.Services[endpointRule.Service]; !ok {
				return fmt.Errorf("Invalid endpoint '%s': service '%s' does not exist.", endpointName, endpointRule.Service)
			} else if !IsPortInService(endpointRule.Port, service.Ports) {
				return fmt.Errorf("Invalid endpoint '%s': service '%s' does not have port '%d'.", endpointName, endpointRule.Service, endpointRule.Port)
			}
		}
	}

	for name, svc := range s.Services {
		if err := validateStackName(name); err != nil {
			return fmt.Errorf("Invalid service name '%s': %s", name, err)
		}

		if svc.Image == "" && svc.Build == nil {
			return fmt.Errorf(fmt.Sprintf("Invalid service '%s': image cannot be empty", name))
		}

		for _, v := range svc.Volumes {
			if v.LocalPath != "" && !isInVolumesTopLevelSection(v.LocalPath, s) {
				s.VolumeMountWarnings = append(s.VolumeMountWarnings, fmt.Sprintf("[%s]: volume '%s:%s' will be ignored. You can synchronize code to your containers using 'okteto up'. More information available here: https://okteto.com/docs/reference/cli/index.html#up", name, v.LocalPath, v.RemotePath))
			}
			if !strings.HasPrefix(v.RemotePath, "/") {
				return fmt.Errorf(fmt.Sprintf("Invalid volume '%s' in service '%s': must be an absolute path", v.ToString(), name))
			}
		}
	}

	return nil
}

func IsPortInService(port int32, portList []Port) bool {
	for _, p := range portList {
		if p.Port == port {
			return true
		}
	}
	return false
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

//SetLastBuiltAnnotation sets the dev timestamp
func (svc *Service) SetLastBuiltAnnotation() {
	if svc.Annotations == nil {
		svc.Annotations = Annotations{}
	}
	svc.Annotations[labels.LastBuiltAnnotation] = time.Now().UTC().Format(labels.TimeFormat)
}

//extendPorts adds the ports that are in expose field to the port list.
func (svc *Service) extendPorts() {
	for _, port := range svc.Expose {
		if !svc.isAlreadyAdded(port) {
			svc.Ports = append(svc.Ports, Port{Port: port, Protocol: apiv1.ProtocolTCP})
		}
	}
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

func GroupWarningsBySvc(fields []string) []string {
	notSupportedMap := make(map[string][]string)
	result := make([]string, 0)
	for _, field := range fields {

		if strings.Contains(field, "[") {
			bracketStart := strings.Index(field, "[")
			bracketEnds := strings.Index(field, "]")

			svcName := field[bracketStart+1 : bracketEnds]

			beforeBrackets := field[:bracketStart]
			afterBrackets := field[bracketEnds+1:]
			field = beforeBrackets + "[%s]" + afterBrackets
			if elem, ok := notSupportedMap[field]; ok {
				elem = append(elem, svcName)
				notSupportedMap[field] = elem
			} else {
				notSupportedMap[field] = []string{svcName}
			}
		} else {
			result = append(result, field)
		}
	}
	for f, svcNames := range notSupportedMap {
		names := strings.Join(svcNames, ", ")
		result = append(result, fmt.Sprintf(f, names))
	}
	return result
}

func isInVolumesTopLevelSection(volumeName string, s *Stack) bool {
	for _, volume := range s.Volumes {
		if volume.Name == volumeName {
			return true
		}
	}
	return false
}
