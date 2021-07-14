// Copyright 2021 The Okteto Authors
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
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/okteto/okteto/pkg/log"
	yaml "gopkg.in/yaml.v2"
	apiv1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
)

var (
	errBadStackName = "must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character"
)

// Stack represents an okteto stack
type Stack struct {
	Manifest  []byte                 `yaml:"-"`
	Warnings  StackWarnings          `yaml:"-"`
	IsCompose bool                   `yaml:"-"`
	Name      string                 `yaml:"name"`
	Volumes   map[string]*VolumeSpec `yaml:"volumes,omitempty"`
	Namespace string                 `yaml:"namespace,omitempty"`
	Services  map[string]*Service    `yaml:"services,omitempty"`
	Endpoints EndpointSpec           `yaml:"endpoints,omitempty"`
}

// Service represents an okteto stack service
type Service struct {
	Build      *BuildInfo         `yaml:"build,omitempty"`
	CapAdd     []apiv1.Capability `yaml:"cap_add,omitempty"`
	CapDrop    []apiv1.Capability `yaml:"cap_drop,omitempty"`
	Entrypoint Entrypoint         `yaml:"entrypoint,omitempty"`
	Command    Command            `yaml:"command,omitempty"`
	EnvFiles   EnvFiles           `yaml:"env_file,omitempty"`
	DependsOn  DependsOn          `yaml:"depends_on,omitempty"`

	Environment     Environment         `yaml:"environment,omitempty"`
	Image           string              `yaml:"image,omitempty"`
	Labels          Labels              `json:"labels,omitempty" yaml:"labels,omitempty"`
	Annotations     Annotations         `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Ports           []Port              `yaml:"ports,omitempty"`
	RestartPolicy   apiv1.RestartPolicy `yaml:"restart,omitempty"`
	StopGracePeriod int64               `yaml:"stop_grace_period,omitempty"`
	Volumes         []StackVolume       `yaml:"volumes,omitempty"`
	Workdir         string              `yaml:"workdir,omitempty"`
	BackOffLimit    int32               `yaml:"max_attempts,omitempty"`
	Healtcheck      *HealthCheck        `yaml:"healthcheck,omitempty"`

	Public    bool            `yaml:"public,omitempty"`
	Replicas  int32           `yaml:"replicas,omitempty"`
	Resources *StackResources `yaml:"resources,omitempty"`

	VolumeMounts []StackVolume `yaml:"-"`
}

type StackVolume struct {
	LocalPath  string
	RemotePath string
}

type VolumeSpec struct {
	Labels      Labels      `yaml:"labels,omitempty"`
	Annotations Annotations `yaml:"annotations,omitempty"`
	Size        Quantity    `json:"size,omitempty" yaml:"size,omitempty"`
	Class       string      `json:"class,omitempty" yaml:"class,omitempty"`
}
type Envs struct {
	List Environment
}
type HealthCheck struct {
	HTTP        *HTTPHealtcheck `yaml:"http,omitempty"`
	Test        HealtcheckTest  `yaml:"test,omitempty"`
	Interval    time.Duration   `yaml:"interval,omitempty"`
	Timeout     time.Duration   `yaml:"timeout,omitempty"`
	Retries     int             `yaml:"retries,omitempty"`
	StartPeriod time.Duration   `yaml:"start_period,omitempty"`
	Disable     bool            `yaml:"disable,omitempty"`
}

type HTTPHealtcheck struct {
	Path string `yaml:"path,omitempty"`
	Port int32  `yaml:"port,omitempty"`
}

type HealtcheckTest []string

// StackResources represents an okteto stack resources
type StackResources struct {
	Limits   ServiceResources `json:"limits,omitempty" yaml:"limits,omitempty"`
	Requests ServiceResources `json:"requests,omitempty" yaml:"requests,omitempty"`
}

// ServiceResources represents an okteto stack service resources
type ServiceResources struct {
	CPU     Quantity        `json:"cpu,omitempty" yaml:"cpu,omitempty"`
	Memory  Quantity        `json:"memory,omitempty" yaml:"memory,omitempty"`
	Storage StorageResource `json:"storage,omitempty" yaml:"storage,omitempty"`
}

// StorageResource represents an okteto stack service storage resource
type StorageResource struct {
	Size  Quantity `json:"size,omitempty" yaml:"size,omitempty"`
	Class string   `json:"class,omitempty" yaml:"class,omitempty"`
}

// Quantity represents an okteto stack service storage resource
type Quantity struct {
	Value resource.Quantity
}

type Port struct {
	HostPort      int32
	ContainerPort int32
	Protocol      apiv1.Protocol
}

type EndpointSpec map[string]Endpoint

// Endpoints represents an okteto stack ingress
type Endpoint struct {
	Labels      Labels         `json:"labels,omitempty" yaml:"labels,omitempty"`
	Annotations Annotations    `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Rules       []EndpointRule `yaml:"rules,omitempty"`
}

// CommandStack represents an okteto stack command
type CommandStack struct {
	Values []string
}

// ArgsStack represents an okteto stack args
type ArgsStack struct {
	Values []string
}

// EndpointRule represents an okteto ingress rule
type EndpointRule struct {
	Path    string `yaml:"path,omitempty"`
	Service string `yaml:"service,omitempty"`
	Port    int32  `yaml:"port,omitempty"`
}

type StackWarnings struct {
	NotSupportedFields  []string          `yaml:"-"`
	SanitizedServices   map[string]string `yaml:"-"`
	VolumeMountWarnings []string          `yaml:"-"`
}
type DependsOn map[string]DependsOnConditionSpec

type DependsOnConditionSpec struct {
	Condition DependsOnCondition `json:"condition,omitempty" yaml:"condition,omitempty"`
}

type DependsOnCondition string

const (
	DependsOnServiceHealthy DependsOnCondition = "service_healthy"

	DependsOnServiceRunning DependsOnCondition = "service_started"

	DependsOnServiceCompleted DependsOnCondition = "service_completed_successfully"
)

// GetStack returns an okteto stack object from a given file
func GetStack(name, stackPath string, isCompose bool) (*Stack, error) {
	b, err := ioutil.ReadFile(stackPath)
	if err != nil {
		return nil, err
	}

	s, err := ReadStack(b, isCompose)
	if err != nil {
		return nil, err
	}

	s.Name, err = getStackName(name, stackPath, s.Name)
	if err != nil {
		return nil, err
	}

	if endpoint, ok := s.Endpoints[""]; ok {
		s.Endpoints[s.Name] = endpoint
		delete(s.Endpoints, "")
	}

	if err := s.validate(); err != nil {
		return nil, err
	}

	stackDir, err := filepath.Abs(filepath.Dir(stackPath))
	if err != nil {
		return nil, err
	}

	for _, svc := range s.Services {
		if svc.Build == nil {
			continue
		}

		if uri, err := url.ParseRequestURI(svc.Build.Context); err == nil || (uri != nil && (uri.Scheme != "" || uri.Host != "")) {
			svc.Build.Dockerfile = ""
		} else {
			svc.Build.Context = loadAbsPath(stackDir, svc.Build.Context)
			svc.Build.Dockerfile = loadAbsPath(stackDir, svc.Build.Dockerfile)
		}
	}
	return s, nil
}

func getStackName(name, stackPath, actualStackName string) (string, error) {
	if name != "" {
		return name, nil
	}
	if actualStackName == "" {
		name, err := GetValidNameFromGitRepo(filepath.Dir(stackPath))
		if err != nil {
			name, err = GetValidNameFromFolder(filepath.Dir(stackPath))
			if err != nil {
				return "", err
			}
		}
		return name, nil
	}
	return actualStackName, nil
}

// ReadStack reads an okteto stack
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

			_, _ = sb.WriteString("    See https://okteto.com/docs/reference/stacks/ for details")
			return nil, errors.New(sb.String())
		}

		msg := strings.Replace(err.Error(), "yaml: unmarshal errors:", "invalid stack manifest:", 1)
		msg = strings.TrimSuffix(msg, "in type model.Stack")
		return nil, errors.New(msg)
	}
	for svcName, svc := range s.Services {
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

		if svc.IsJob() {
			for idx, volume := range svc.Volumes {
				volumeName := fmt.Sprintf("pvc-%s-0", svcName)
				if volume.LocalPath == "" {
					volume.LocalPath = volumeName
					s.Volumes[volumeName] = &VolumeSpec{Size: svc.Resources.Requests.Storage.Size}
				}
				svc.Volumes[idx] = volume

			}
		}
	}
	for _, volume := range s.Volumes {
		if volume.Size.Value.Cmp(resource.MustParse("0")) == 0 {
			volume.Size.Value = resource.MustParse("1Gi")
		}
	}
	return s, nil
}

func (svc *Service) IgnoreSyncVolumes(s *Stack) {
	notIgnoredVolumes := make([]StackVolume, 0)
	for _, volume := range svc.VolumeMounts {
		if !strings.HasPrefix(volume.LocalPath, "/") {
			notIgnoredVolumes = append(notIgnoredVolumes, volume)
		}
	}
	svc.VolumeMounts = notIgnoredVolumes
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
			if service, ok := s.Services[endpointRule.Service]; ok {
				if !IsPortInService(endpointRule.Port, service.Ports) {
					return fmt.Errorf("Invalid endpoint '%s': service '%s' does not have port '%d'.", endpointName, endpointRule.Service, endpointRule.Port)
				}
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

		for _, v := range svc.VolumeMounts {
			if strings.HasPrefix(v.LocalPath, "/") {
				s.Warnings.VolumeMountWarnings = append(s.Warnings.VolumeMountWarnings, fmt.Sprintf("[%s]: volume '%s:%s' will be ignored. You can synchronize code to your containers using 'okteto up'. More information available here: https://okteto.com/docs/reference/cli/#up", name, v.LocalPath, v.RemotePath))
			}
			if !strings.HasPrefix(v.RemotePath, "/") {
				return fmt.Errorf(fmt.Sprintf("Invalid volume '%s' in service '%s': must be an absolute path", v.ToString(), name))
			}
		}
		svc.IgnoreSyncVolumes(s)
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
	return fmt.Sprintf("%s=%s", StackNameLabel, s.Name)
}

//GetLabelSelector returns the label selector for the stack name
func GetStackConfigMapName(stackName string) string {
	return fmt.Sprintf("okteto-%s", stackName)
}

func IsPortInService(port int32, ports []Port) bool {
	for _, p := range ports {
		if p.ContainerPort == port {
			return true
		}
	}
	return false
}

//SetLastBuiltAnnotation sets the dev timestamp
func (svc *Service) SetLastBuiltAnnotation() {
	if svc.Annotations == nil {
		svc.Annotations = Annotations{}
	}
	svc.Annotations[LastBuiltAnnotation] = time.Now().UTC().Format(TimeFormat)
}

//isAlreadyAdded checks if a port is already on port list
func IsAlreadyAdded(p Port, ports []Port) bool {
	for _, port := range ports {
		if port.ContainerPort == p.ContainerPort {
			log.Infof("Port '%d:%d' is already declared on port '%d:%d'", p.HostPort, p.HostPort, port.HostPort, port.ContainerPort)
			return true
		}
	}
	return false
}

func IsAlreadyAddedExpose(p Port, ports []Port) bool {
	for _, port := range ports {
		if p.HostPort == 0 && (port.ContainerPort == p.ContainerPort || port.HostPort == p.ContainerPort) {
			log.Infof("Expose port '%d:%d' is already declared on port '%d:%d'", p.HostPort, p.HostPort, port.HostPort, port.ContainerPort)
			return true

		} else if p.HostPort != 0 && (port.ContainerPort == p.ContainerPort || port.ContainerPort == p.HostPort || port.HostPort == p.HostPort || port.HostPort == p.ContainerPort) {
			log.Infof("Expose port '%d:%d' is already declared on port '%d:%d'", p.HostPort, p.HostPort, port.HostPort, port.ContainerPort)
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
	_, ok := s.Volumes[volumeName]
	return ok
}

func (svc *Service) IsDeployment() bool {
	return len(svc.Volumes) == 0 && (svc.RestartPolicy == apiv1.RestartPolicyAlways || (svc.RestartPolicy == apiv1.RestartPolicyOnFailure && svc.BackOffLimit == 0))
}
func (svc *Service) IsStatefulset() bool {
	return len(svc.Volumes) != 0 && (svc.RestartPolicy == apiv1.RestartPolicyAlways || (svc.RestartPolicy == apiv1.RestartPolicyOnFailure && svc.BackOffLimit == 0))
}
func (svc *Service) IsJob() bool {
	return svc.RestartPolicy == apiv1.RestartPolicyNever || (svc.RestartPolicy == apiv1.RestartPolicyOnFailure && svc.BackOffLimit != 0)
}
