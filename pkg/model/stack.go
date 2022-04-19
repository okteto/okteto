// Copyright 2022 The Okteto Authors
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
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	yaml "gopkg.in/yaml.v2"
	apiv1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
)

var (
	errBadStackName = "must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character"
	//DefaultStackManifest default okteto stack manifest file
	possibleStackManifests = [][]string{
		{"okteto-stack.yml"},
		{"okteto-stack.yaml"},
		{"stack.yml"},
		{"stack.yaml"},
		{".okteto", "okteto-stack.yml"},
		{".okteto", "okteto-stack.yaml"},
		{"okteto-compose.yml"},
		{"okteto-compose.yaml"},
		{".okteto", "okteto-compose.yml"},
		{".okteto", "okteto-compose.yaml"},
		{"docker-compose.yml"},
		{"docker-compose.yaml"},
		{".okteto", "docker-compose.yml"},
		{".okteto", "docker-compose.yaml"},
	}
	deprecatedManifests = []string{"stack.yml", "stack.yaml"}
)

// Stack represents an okteto stack
type Stack struct {
	Manifest  []byte                 `yaml:"-"`
	Warnings  StackWarnings          `yaml:"-"`
	IsCompose bool                   `yaml:"-"`
	Name      string                 `yaml:"name"`
	Volumes   map[string]*VolumeSpec `yaml:"volumes,omitempty"`
	Namespace string                 `yaml:"namespace,omitempty"`
	Context   string                 `yaml:"context,omitempty"`
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

	Environment     Environment           `yaml:"environment,omitempty"`
	Image           string                `yaml:"image,omitempty"`
	Labels          Labels                `json:"labels,omitempty" yaml:"labels,omitempty"`
	Annotations     Annotations           `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Ports           []Port                `yaml:"ports,omitempty"`
	RestartPolicy   apiv1.RestartPolicy   `yaml:"restart,omitempty"`
	StopGracePeriod int64                 `yaml:"stop_grace_period,omitempty"`
	Volumes         []StackVolume         `yaml:"volumes,omitempty"`
	Workdir         string                `yaml:"workdir,omitempty"`
	BackOffLimit    int32                 `yaml:"max_attempts,omitempty"`
	Healtcheck      *HealthCheck          `yaml:"healthcheck,omitempty"`
	User            *StackSecurityContext `yaml:"user,omitempty"`

	Public    bool            `yaml:"public,omitempty"`
	Replicas  int32           `yaml:"replicas,omitempty"`
	Resources *StackResources `yaml:"resources,omitempty"`

	VolumeMounts []StackVolume `yaml:"-"`
}

// StackSecurityContext defines which user and group use
type StackSecurityContext struct {
	RunAsUser  *int64 `json:"runAsUser,omitempty" yaml:"runAsUser,omitempty"`
	RunAsGroup *int64 `json:"runAsGroup,omitempty" yaml:"runAsGroup,omitempty"`
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

// GetStackFromPath returns an okteto stack object from a given file
func GetStackFromPath(name, stackPath string, isCompose bool) (*Stack, error) {
	b, err := os.ReadFile(stackPath)
	if err != nil {
		return nil, err
	}

	expandedManifest, err := ExpandEnv(string(b), true)
	if err != nil {
		return nil, err
	}
	s, err := ReadStack([]byte(expandedManifest), isCompose)
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
			svc.Build.Dockerfile = loadAbsPath(svc.Build.Context, svc.Build.Dockerfile)
		}
		copy(svc.Build.VolumesToInclude, svc.Volumes)
	}
	return s, nil
}

func getStackName(name, stackPath, actualStackName string) (string, error) {
	if name != "" {
		if err := os.Setenv(OktetoNameEnvVar, name); err != nil {
			return "", err
		}
		return name, nil
	}
	if actualStackName == "" {
		nameEnvVar := os.Getenv(OktetoNameEnvVar)
		if nameEnvVar != "" {
			return nameEnvVar, nil
		}
		name, err := GetValidNameFromGitRepo(filepath.Dir(stackPath))
		if err != nil {
			name, err = GetValidNameFromFolder(filepath.Dir(stackPath))
			if err != nil {
				return "", err
			}
		}
		if err := os.Setenv(OktetoNameEnvVar, name); err != nil {
			return "", err
		}
		return name, nil
	}
	if err := os.Setenv(OktetoNameEnvVar, actualStackName); err != nil {
		return "", err
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
			_, _ = sb.WriteString("Invalid compose manifest:\n")
			l := strings.Split(err.Error(), "\n")
			for i := 1; i < len(l); i++ {
				e := strings.TrimSuffix(l[i], "in type model.Stack")
				e = strings.TrimSpace(e)
				_, _ = sb.WriteString(fmt.Sprintf("    - %s\n", e))
			}

			_, _ = sb.WriteString("    See https://okteto.com/docs/reference/compose/ for details")
			return nil, errors.New(sb.String())
		}

		msg := strings.Replace(err.Error(), "yaml: unmarshal errors:", "invalid compose manifest:", 1)
		msg = strings.TrimSuffix(msg, "in type model.Stack")
		return nil, errors.New(msg)
	}
	for svcName, svc := range s.Services {
		if svc.Build != nil {
			if svc.Build.Name != "" {
				svc.Build.Context = svc.Build.Name
				svc.Build.Name = ""
			}
			svc.Build.setBuildDefaults()
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

// ToDev translates a service into a dev
func (svc *Service) ToDev(svcName string) (*Dev, error) {
	d := NewDev()
	for _, p := range svc.Ports {
		if p.HostPort != 0 {
			d.Forward = append(d.Forward, Forward{Local: int(p.HostPort), Remote: int(p.ContainerPort)})
		}
	}
	for _, v := range svc.VolumeMounts {
		if pathExistsAndDir(v.LocalPath) {
			d.Sync.Folders = append(d.Sync.Folders, SyncFolder(v))
		}
	}
	d.Command = svc.Command
	d.EnvFiles = svc.EnvFiles
	d.Environment = svc.Environment
	d.Name = svcName
	err := d.SetDefaults()
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (s *Stack) Validate() error {
	if err := validateStackName(s.Name); err != nil {
		return fmt.Errorf("Invalid compose name: %s", err)
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
	return validateDependsOn(s)
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

func validateDependsOn(s *Stack) error {
	for svcName, svc := range s.Services {
		for dependentSvc, condition := range svc.DependsOn {
			if svcName == dependentSvc {
				return fmt.Errorf(" Service '%s' depends can not depend of itself.", svcName)
			}
			if _, ok := s.Services[dependentSvc]; !ok {
				return fmt.Errorf(" Service '%s' depends on service '%s' which is undefined.", svcName, dependentSvc)
			}
			if condition.Condition == DependsOnServiceCompleted && !s.Services[dependentSvc].IsJob() {
				return fmt.Errorf(" Service '%s' is not a job. Please make sure the 'restart_policy' is not set to 'always' in service '%s' ", dependentSvc, dependentSvc)
			}
		}
	}

	dependencyCycle := getDependentCyclic(s)
	if len(dependencyCycle) > 0 {
		svcsDependents := fmt.Sprintf("%s and %s", strings.Join(dependencyCycle[:len(dependencyCycle)-1], ", "), dependencyCycle[len(dependencyCycle)-1])
		return fmt.Errorf(" There was a cyclic dependendecy between %s.", svcsDependents)
	}
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
			oktetoLog.Infof("Port '%d:%d' is already declared on port '%d:%d'", p.HostPort, p.HostPort, port.HostPort, port.ContainerPort)
			return true
		}
	}
	return false
}

func IsAlreadyAddedExpose(p Port, ports []Port) bool {
	for _, port := range ports {
		if p.HostPort == 0 && (port.ContainerPort == p.ContainerPort || port.HostPort == p.ContainerPort) {
			oktetoLog.Infof("Expose port '%d:%d' is already declared on port '%d:%d'", p.HostPort, p.HostPort, port.HostPort, port.ContainerPort)
			return true

		} else if p.HostPort != 0 && (port.ContainerPort == p.ContainerPort || port.ContainerPort == p.HostPort || port.HostPort == p.HostPort || port.HostPort == p.ContainerPort) {
			oktetoLog.Infof("Expose port '%d:%d' is already declared on port '%d:%d'", p.HostPort, p.HostPort, port.HostPort, port.ContainerPort)
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

func (stack *Stack) Merge(otherStack *Stack) *Stack {
	if stack == nil {
		return otherStack
	}
	if otherStack.Namespace != "" {
		stack.Namespace = otherStack.Namespace
	}
	if len(otherStack.Endpoints) > 0 {
		stack.Endpoints = otherStack.Endpoints
	}
	if len(otherStack.Volumes) > 0 {
		stack.Volumes = otherStack.Volumes
	}
	stack = stack.mergeServices(otherStack)
	return stack
}

func (stack *Stack) mergeServices(otherStack *Stack) *Stack {
	for svcName, svc := range otherStack.Services {
		if _, ok := stack.Services[svcName]; !ok {
			stack.Services[svcName] = svc
			continue
		}
		resultSvc := stack.Services[svcName]
		if svc.Image != "" {
			resultSvc.Image = svc.Image
		}
		if svc.RestartPolicy != apiv1.RestartPolicyAlways {
			resultSvc.RestartPolicy = svc.RestartPolicy
		}
		if svc.Workdir != "" {
			resultSvc.Workdir = svc.Workdir
		}
		if svc.Replicas != 1 {
			resultSvc.Replicas = svc.Replicas
		}
		if svc.StopGracePeriod != 0 {
			resultSvc.StopGracePeriod = svc.StopGracePeriod
		}
		if svc.BackOffLimit != 0 {
			resultSvc.BackOffLimit = svc.BackOffLimit
		}
		if svc.Build != nil {
			resultSvc.Build = svc.Build
		}
		if svc.Healtcheck != nil {
			resultSvc.Healtcheck = svc.Healtcheck
		}

		if len(svc.CapAdd) > 0 {
			resultSvc.CapAdd = svc.CapAdd
		}
		if len(svc.CapDrop) > 0 {
			resultSvc.CapDrop = svc.CapDrop
		}

		if len(svc.Entrypoint.Values) > 0 {
			resultSvc.Entrypoint = svc.Entrypoint
		}
		if len(svc.Command.Values) > 0 {
			resultSvc.Command = svc.Command
		}
		if len(svc.EnvFiles) > 0 {
			resultSvc.EnvFiles = svc.EnvFiles
		}
		if len(svc.DependsOn) > 0 {
			resultSvc.DependsOn = svc.DependsOn
		}
		if len(svc.Environment) > 0 {
			resultSvc.Environment = svc.Environment
		}
		if len(svc.Labels) > 0 {
			resultSvc.Labels = svc.Labels
		}
		if len(svc.Annotations) > 0 {
			resultSvc.Annotations = svc.Annotations
		}
		if len(svc.Ports) > 0 {
			resultSvc.Ports = svc.Ports
		}
		if len(svc.Volumes) > 0 {
			resultSvc.Volumes = svc.Volumes
			resultSvc.VolumeMounts = svc.VolumeMounts
		}
		if !svc.Resources.IsDefaultValue() {
			resultSvc.Resources = svc.Resources
		}
	}
	return stack
}

func (r *StackResources) IsDefaultValue() bool {
	if r == nil {
		return true
	}
	if r.Limits.IsDefaultValue() && r.Requests.IsDefaultValue() {
		return true
	}
	return false
}

func (svcResources *ServiceResources) IsDefaultValue() bool {
	return svcResources.CPU.Value.IsZero() && svcResources.Memory.Value.IsZero() && svcResources.Storage.Size.Value.IsZero() && svcResources.Storage.Class == ""
}

func isPathAComposeFile(path string) bool {
	base := filepath.Base(path)
	return strings.HasPrefix(base, "docker-compose") || strings.HasPrefix(base, "okteto-compose")
}

// LoadStack loads an okteto stack manifest checking "yml" and "yaml"
func LoadStack(name string, stackPaths []string, validate bool) (*Stack, error) {
	var resultStack *Stack

	if len(stackPaths) == 0 {
		stack, err := inferStack(name)
		if err != nil {
			return nil, err
		}
		resultStack = resultStack.Merge(stack)

	} else {
		for _, stackPath := range stackPaths {
			if FileExists(stackPath) {
				stack, err := getStack(name, stackPath)
				if err != nil {
					return nil, err
				}

				resultStack = resultStack.Merge(stack)
				continue
			}
			return nil, fmt.Errorf("'%s' does not exist", stackPath)
		}
	}
	if validate {
		if err := resultStack.Validate(); err != nil {
			return nil, err
		}
	}

	return resultStack, nil
}

func inferStack(name string) (*Stack, error) {
	for _, possibleStackManifest := range possibleStackManifests {
		manifestPath := filepath.Join(possibleStackManifest...)
		if FileExists(manifestPath) {
			stack, err := getStack(name, manifestPath)
			if err != nil {
				return nil, err
			}

			return stack, nil
		}
	}
	return nil, oktetoErrors.UserError{
		E:    fmt.Errorf("could not detect any compose file"),
		Hint: "If you have a compose file, use the flag '--file' to point to your compose file",
	}
}

func getStack(name, manifestPath string) (*Stack, error) {
	var isCompose bool
	if isDeprecatedExtension(manifestPath) {
		deprecatedFile := filepath.Base(manifestPath)
		oktetoLog.Warning("The file %s will be deprecated as a default compose file name in a future version. Please consider renaming your compose file to 'okteto-stack.yml'", deprecatedFile)
	}
	if isPathAComposeFile(manifestPath) {
		isCompose = true
	}
	stack, err := GetStackFromPath(name, manifestPath, isCompose)
	if err != nil {
		return nil, err
	}
	overrideStack, err := getOverrideFile(manifestPath)
	if err == nil {
		oktetoLog.Info("override file detected. Merging it")
		stack = stack.Merge(overrideStack)
	}
	return stack, nil
}

func isDeprecatedExtension(stackPath string) bool {
	base := filepath.Base(stackPath)
	for _, deprecatedManifest := range deprecatedManifests {
		if deprecatedManifest == base {
			return true
		}
	}
	return false
}

func getOverrideFile(stackPath string) (*Stack, error) {
	extension := filepath.Ext(stackPath)
	fileName := strings.TrimSuffix(stackPath, extension)
	overridePath := fmt.Sprintf("%s.override%s", fileName, extension)
	var isCompose bool
	if FileExists(stackPath) {
		if isPathAComposeFile(stackPath) {
			isCompose = true
		}
		stack, err := GetStackFromPath("", overridePath, isCompose)
		if err != nil {
			return nil, err
		}
		return stack, nil
	}
	return nil, fmt.Errorf("override file not found")
}
