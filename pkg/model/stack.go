// Copyright 2023 The Okteto Authors
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
	"sort"
	"strings"
	"time"

	"github.com/compose-spec/godotenv"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/discovery"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/filesystem"
	"github.com/okteto/okteto/pkg/format"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model/forward"
	yaml "gopkg.in/yaml.v2"
	apiv1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
)

var (
	errBadStackName     = "must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character"
	deprecatedManifests = []string{"stack.yml", "stack.yaml"}
	errDependsOn        = errors.New("invalid depends_on")
)

// Stack represents an okteto stack
type Stack struct {
	Manifest  []byte                 `yaml:"-"`
	Paths     []string               `yaml:"-"`
	Warnings  StackWarnings          `yaml:"-"`
	IsCompose bool                   `yaml:"-"`
	Name      string                 `yaml:"name"`
	Volumes   map[string]*VolumeSpec `yaml:"volumes,omitempty"`
	Namespace string                 `yaml:"namespace,omitempty"`
	Context   string                 `yaml:"context,omitempty"`
	Services  ComposeServices        `yaml:"services,omitempty"`
	Endpoints EndpointSpec           `yaml:"endpoints,omitempty"`
}

// ComposeServices represents the services declared in the compose
type ComposeServices map[string]*Service

func (cs ComposeServices) getNames() []string {
	names := []string{}
	for k := range cs {
		names = append(names, k)
	}
	return names
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
	NodeSelector    Selector              `json:"x-node-selector,omitempty" yaml:"x-node-selector,omitempty"`
	Ports           []Port                `yaml:"ports,omitempty"`
	RestartPolicy   apiv1.RestartPolicy   `yaml:"restart,omitempty"`
	StopGracePeriod int64                 `yaml:"stop_grace_period,omitempty"`
	Volumes         []StackVolume         `yaml:"volumes,omitempty"`
	Workdir         string                `yaml:"workdir,omitempty"`
	BackOffLimit    int32                 `yaml:"max_attempts,omitempty"`
	Healtcheck      *HealthCheck          `yaml:"healthcheck,omitempty"`
	User            *StackSecurityContext `yaml:"user,omitempty"`

	// Fields only for okteto stacks
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
	Liveness    bool            `yaml:"x-okteto-liveness,omitempty"`
	Readiness   bool            `default:"true" yaml:"x-okteto-readiness,omitempty"`
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

type PortInterface interface {
	GetHostPort() int32
	GetContainerPort() int32
	GetProtocol() apiv1.Protocol
}

type Port struct {
	HostPort      int32
	ContainerPort int32
	Protocol      apiv1.Protocol
}

func (p Port) GetHostPort() int32          { return p.HostPort }
func (p Port) GetContainerPort() int32     { return p.ContainerPort }
func (p Port) GetProtocol() apiv1.Protocol { return p.Protocol }

type EndpointSpec map[string]Endpoint

// Endpoint represents an okteto stack ingress
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

	if isEmptyManifestFile(b) {
		return nil, fmt.Errorf("%w: %s", oktetoErrors.ErrInvalidManifest, oktetoErrors.ErrEmptyManifest)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := os.Chdir(cwd); err != nil {
			oktetoLog.Infof("failed to change directory to %s: %s", cwd, err)
		}
	}()

	stackWorkingDir := GetWorkdirFromManifestPath(stackPath)
	if err := os.Chdir(stackWorkingDir); err != nil {
		return nil, err
	}
	stackPath = GetManifestPathFromWorkdir(stackPath, stackWorkingDir)

	s, err := ReadStack(b, isCompose)
	if err != nil {
		return nil, err
	}
	s.Paths = []string{stackPath}
	s.Name, err = getStackName(name, stackPath, s.Name)
	if err != nil {
		return nil, err
	}

	if endpoint, ok := s.Endpoints[""]; ok {
		// s.Name should be sanitize and compliant with url format
		s.Endpoints[format.ResourceK8sMetaString(s.Name)] = endpoint
		delete(s.Endpoints, "")
	}

	stackDir, err := filepath.Abs(filepath.Dir(stackPath))
	if err != nil {
		return nil, err
	}

	for svcName, svc := range s.Services {
		if err := loadEnvFiles(svc, svcName); err != nil {
			return nil, err
		}
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

// getStackName it returns the stack name based in the following criteria
//   - If `name` is set, that value is used as stack name. This represents the value provided by
//     the user with `--name`
//   - If `actualStackName` is provided, that is the value used. This represents the value provided
//     in the okteto-stack file with the property `name`
//   - If none of them is provided, we get the name from the repository (if any) in the folder where
//     the stack/compose file is (`stackPath`)
//   - If no repository is found, we get the name from the folder where the stack/compose file is (`stackPath`)
func getStackName(name, stackPath, actualStackName string) (string, error) {
	if name != "" {
		if err := os.Setenv(constants.OktetoNameEnvVar, name); err != nil {
			return "", err
		}
		return name, nil
	}
	if actualStackName == "" {
		nameEnvVar := os.Getenv(constants.OktetoNameEnvVar)
		if nameEnvVar != "" {
			// this name could be not sanitized when running at pipeline installer
			return nameEnvVar, nil
		}
		name, err := GetValidNameFromGitRepo(filepath.Dir(stackPath))
		if err != nil {
			name, err = GetValidNameFromFolder(filepath.Dir(stackPath))
			if err != nil {
				return "", err
			}
		}
		if err := os.Setenv(constants.OktetoNameEnvVar, name); err != nil {
			return "", err
		}
		return name, nil
	}
	if err := os.Setenv(constants.OktetoNameEnvVar, actualStackName); err != nil {
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
	expandedManifest, err := ExpandStackEnvs(bytes)
	if err != nil {
		return nil, err
	}

	if err := yaml.UnmarshalStrict(expandedManifest, s); err != nil {
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
		if errors.Is(err, oktetoErrors.ErrServiceEmpty) {
			return nil, err
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

func (svc *Service) ignoreSyncVolumes() {
	notIgnoredVolumes := make([]StackVolume, 0)
	wd, err := os.Getwd()
	if err != nil {
		oktetoLog.Info("could not get wd to ignore secrets")
	}
	for _, volume := range svc.VolumeMounts {
		if filepath.IsAbs(volume.LocalPath) {
			relPath, err := filepath.Rel(wd, volume.LocalPath)
			if err != nil {
				oktetoLog.Infof("could not get rel: %s", err)
			}
			volume.LocalPath = relPath
		}
		if filesystem.FileExists(volume.LocalPath) {
			notIgnoredVolumes = append(notIgnoredVolumes, volume)
			continue
		}
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
			d.Forward = append(d.Forward, forward.Forward{Local: int(p.HostPort), Remote: int(p.ContainerPort)})
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
	// in case name is coming from option "name" at deploy this could not be sanitized
	if err := validateStackName(format.ResourceK8sMetaString(s.Name)); err != nil {
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

	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	for name, svc := range s.Services {
		if err := validateStackName(name); err != nil {
			return fmt.Errorf("Invalid service name '%s': %s", name, err)
		}

		if svc.Image == "" && svc.Build == nil {
			return fmt.Errorf(fmt.Sprintf("Invalid service '%s': image cannot be empty", name))
		}

		for _, v := range svc.VolumeMounts {
			if svc.Build == nil && filesystem.FileExists(v.LocalPath) {
				continue
			}
			if _, err := filepath.Rel(wd, v.LocalPath); err != nil {
				s.Warnings.VolumeMountWarnings = append(s.Warnings.VolumeMountWarnings, fmt.Sprintf("[%s]: volume '%s:%s' will be ignored. You can synchronize code to your containers using 'okteto up'. More information available here: https://okteto.com/docs/reference/cli/#up", name, v.LocalPath, v.RemotePath))
			}
			if !strings.HasPrefix(v.RemotePath, "/") {
				return fmt.Errorf(fmt.Sprintf("Invalid volume '%s' in service '%s': must be an absolute path", v.ToString(), name))
			}
		}
		svc.ignoreSyncVolumes()
	}
	return s.Services.ValidateDependsOn(s.Services.getNames())
}

// validateStackName checks if the name is compliant
// name param is sanitized
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

type errDependsOnItself struct {
	service string
}

func (e *errDependsOnItself) Error() string {
	return fmt.Sprintf("%s: Service '%s' depends cannot depend of itself.", errDependsOn.Error(), e.service)
}

func (e *errDependsOnItself) Unwrap() error {
	return errDependsOn
}

type errDependsOnUndefined struct {
	svc          string
	dependentSvc string
}

func (e *errDependsOnUndefined) Error() string {
	return fmt.Errorf("%w: Service '%s' depends on service '%s' which is undefined.", errDependsOn, e.svc, e.dependentSvc).Error()
}

func (e *errDependsOnUndefined) Unwrap() error {
	return errDependsOn
}

func (cs ComposeServices) ValidateDependsOn(svcs []string) error {
	for _, svcName := range svcs {
		svc, ok := cs[svcName]
		if !ok {
			return fmt.Errorf("service '%s' is not defined in the stack", svcName)
		}
		for dependentSvc, condition := range svc.DependsOn {
			if svcName == dependentSvc {
				return &errDependsOnItself{service: svcName}
			}
			if _, ok := cs[dependentSvc]; !ok {
				return &errDependsOnUndefined{svc: svcName, dependentSvc: dependentSvc}
			}
			if condition.Condition == DependsOnServiceCompleted && !cs[dependentSvc].IsJob() {
				return fmt.Errorf("%w: Service '%s' is not a job. Please make sure the 'restart_policy' is not set to 'always' in service '%s' ", errDependsOn, dependentSvc, dependentSvc)
			}
		}
	}

	dependencyCycle := getDependentCyclic(cs.toGraph())
	if len(dependencyCycle) > 0 {
		svcsDependents := fmt.Sprintf("%s and %s", strings.Join(dependencyCycle[:len(dependencyCycle)-1], ", "), dependencyCycle[len(dependencyCycle)-1])
		return fmt.Errorf("%w: There was a cyclic dependendecy between %s.", errDependsOn, svcsDependents)
	}
	return nil
}

// GetLabelSelector returns the label selector for the stack name
func (s *Stack) GetLabelSelector() string {
	// we need to sanitize the stack name in case this is overridden by the deploy options name
	return fmt.Sprintf("%s=%s", StackNameLabel, format.ResourceK8sMetaString(s.Name))
}

// GetStackConfigMapName returns the label selector for the stack name
func GetStackConfigMapName(stackName string) string {
	// we need to sanitize the stack name in case this is overridden by the deploy options name
	return fmt.Sprintf("okteto-%s", format.ResourceK8sMetaString(stackName))
}

func IsPortInService(port int32, ports []Port) bool {
	for _, p := range ports {
		if p.ContainerPort == port {
			return true
		}
	}
	return false
}

// SetLastBuiltAnnotation sets the dev timestamp
func (svc *Service) SetLastBuiltAnnotation() {
	if svc.Annotations == nil {
		svc.Annotations = Annotations{}
	}
	svc.Annotations[LastBuiltAnnotation] = time.Now().UTC().Format(constants.TimeFormat)
}

// IsAlreadyAdded checks if a port is already on port list
func IsAlreadyAdded(p PortInterface, ports []Port) bool {
	for _, port := range ports {
		if port.GetContainerPort() == p.GetContainerPort() {
			oktetoLog.Infof("Port '%d:%d' is already declared on port '%d:%d'", p.GetHostPort(), p.GetContainerPort(), port.GetHostPort(), port.GetContainerPort())
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
	stack.Paths = append(stack.Paths, otherStack.Paths...)
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
		if len(svc.NodeSelector) > 0 {
			resultSvc.NodeSelector = svc.NodeSelector
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
		dir, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		stack, err := inferStack(dir, name)
		if err != nil {
			return nil, err
		}
		resultStack = resultStack.Merge(stack)

	} else {
		for _, stackPath := range stackPaths {
			if filesystem.FileExists(stackPath) {
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

func inferStack(wd, name string) (*Stack, error) {
	composePath, err := discovery.GetComposePath(wd)
	if err != nil {
		return nil, err
	}
	return getStack(name, composePath)
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
	if filesystem.FileExists(stackPath) {
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

func loadEnvFiles(svc *Service, svcName string) error {
	for i := len(svc.EnvFiles) - 1; i >= 0; i-- {
		envFilepath := svc.EnvFiles[i]
		if err := setEnvironmentFromFile(svc, envFilepath); err != nil {
			if filepath.Base(envFilepath) == ".env" {
				oktetoLog.Warning("Skipping '.env' file from %s service", svcName)
				continue
			}
			return err
		}
	}
	sort.SliceStable(svc.Environment, func(i, j int) bool {
		return strings.Compare(svc.Environment[i].Name, svc.Environment[j].Name) < 0
	})
	svc.EnvFiles = nil
	return nil
}

func setEnvironmentFromFile(svc *Service, filename string) error {
	var err error
	filename, err = ExpandEnv(filename, true)
	if err != nil {
		return err
	}

	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer func() {
		if err := f.Close(); err != nil {
			oktetoLog.Debugf("Error closing file %s: %s", filename, err)
		}
	}()

	envMap, err := godotenv.ParseWithLookup(f, os.LookupEnv)
	if err != nil {
		return fmt.Errorf("error parsing env_file %s: %s", filename, err.Error())
	}

	for _, e := range svc.Environment {
		delete(envMap, e.Name)
	}

	for name, value := range envMap {
		if value == "" {
			value = os.Getenv(name)
		}
		svc.Environment = append(
			svc.Environment,
			EnvVar{Name: name, Value: value},
		)
	}

	return nil
}

func (s ComposeServices) toGraph() graph {
	g := graph{}
	for svcName, svcInfo := range s {
		dependsOnList := []string{}
		for dependantSvc := range svcInfo.DependsOn {
			dependsOnList = append(dependsOnList, dependantSvc)
		}
		g[svcName] = dependsOnList
	}
	return g
}

func (stack *Stack) GetServicesWithBuildSection() map[string]bool {
	result := make(map[string]bool)
	for name, service := range stack.Services {
		if service.Build != nil || len(service.VolumeMounts) != 0 {
			result[name] = true
		}
	}
	return result
}
