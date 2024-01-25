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
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/kballard/go-shellquote"
	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/cache"
	"github.com/okteto/okteto/pkg/env"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/filesystem"
	"github.com/okteto/okteto/pkg/model/forward"
	apiv1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
)

const (
	DefaultReplicasNumber = 1
)

// StackRaw represents an okteto stack
type StackRaw struct {
	Version   string                     `yaml:"version,omitempty"`
	Name      string                     `yaml:"name"`
	Namespace string                     `yaml:"namespace,omitempty"`
	Context   string                     `yaml:"context,omitempty"`
	Services  map[string]*ServiceRaw     `yaml:"services,omitempty"`
	Endpoints EndpointSpec               `yaml:"endpoints,omitempty"`
	Volumes   map[string]*VolumeTopLevel `yaml:"volumes,omitempty"`

	// Extensions
	Extensions map[string]interface{} `yaml:",inline" json:"-"`

	// Docker-compose not implemented
	Networks *WarningType `yaml:"networks,omitempty"`

	Configs *WarningType `yaml:"configs,omitempty"`
	Secrets *WarningType `yaml:"secrets,omitempty"`

	Warnings StackWarnings
}

// ServiceRaw represents an okteto stack service
type ServiceRaw struct {
	MemSwappiness            *WarningType           `yaml:"mem_swappiness,omitempty"`
	CredentialSpec           *WarningType           `yaml:"credential_spec,omitempty"`
	Extensions               map[string]interface{} `yaml:",inline" json:"-"`
	VolumesFrom              *WarningType           `yaml:"volumes_from,omitempty"`
	UsernsMode               *WarningType           `yaml:"userns_mode,omitempty"`
	Ulimits                  *WarningType           `yaml:"ulimits,omitempty"`
	Tty                      *WarningType           `yaml:"tty,omitempty"`
	Tmpfs                    *WarningType           `yaml:"tmpfs,omitempty"`
	Sysctls                  *WarningType           `yaml:"sysctls,omitempty"`
	StorageOpts              *WarningType           `yaml:"storage_opts,omitempty"`
	StopSignal               *WarningType           `yaml:"stop_signal,omitempty"`
	StdinOpen                *WarningType           `yaml:"stdin_open,omitempty"`
	ShmSize                  *WarningType           `yaml:"shm_size,omitempty"`
	SecurityOpt              *WarningType           `yaml:"security_opt,omitempty"`
	Secrets                  *WarningType           `yaml:"secrets,omitempty"`
	Healthcheck              *HealthCheck           `yaml:"healthcheck,omitempty"`
	Runtime                  *WarningType           `yaml:"runtime,omitempty"`
	Labels                   Labels                 `json:"labels,omitempty" yaml:"labels,omitempty"`
	Annotations              Annotations            `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	NodeSelector             Selector               `json:"x-node-selector,omitempty" yaml:"x-node-selector,omitempty"`
	ReadOnly                 *WarningType           `yaml:"read_only,omitempty"`
	PullPolicy               *WarningType           `yaml:"pull_policy,omitempty"`
	ContainerName            *WarningType           `yaml:"container_name,omitempty"`
	Profiles                 *WarningType           `yaml:"profiles,omitempty"`
	Scale                    *int32                 `yaml:"scale"`
	StopGracePeriodSneakCase *RawMessage            `yaml:"stop_grace_period,omitempty"`
	StopGracePeriod          *RawMessage            `yaml:"stopGracePeriod,omitempty"`
	User                     *StackSecurityContext  `yaml:"user,omitempty"`
	Privileged               *WarningType           `yaml:"privileged,omitempty"`
	Platform                 *WarningType           `yaml:"platform,omitempty"`
	PidLimit                 *WarningType           `yaml:"pid_limit,omitempty"`
	DependsOn                DependsOn              `yaml:"depends_on,omitempty"`
	Pid                      *WarningType           `yaml:"pid,omitempty"`
	Replicas                 *int32                 `yaml:"replicas"`
	Resources                *StackResources        `yaml:"resources,omitempty"`
	BlkioConfig              *WarningType           `yaml:"blkio_config,omitempty"`
	CpuPercent               *WarningType           `yaml:"cpu_percent,omitempty"`
	CpuShares                *WarningType           `yaml:"cpu_shares,omitempty"`
	CpuPeriod                *WarningType           `yaml:"cpu_period,omitempty"`
	CpuQuota                 *WarningType           `yaml:"cpu_quota,omitempty"`
	CpuRtRuntime             *WarningType           `yaml:"cpu_rt_runtime,omitempty"`
	CpuRtPeriod              *WarningType           `yaml:"cpu_rt_period,omitempty"`
	Cpuset                   *WarningType           `yaml:"cpuset,omitempty"`
	CgroupParent             *WarningType           `yaml:"cgroup_parent,omitempty"`
	Networks                 *WarningType           `yaml:"networks,omitempty"`
	Build                    *composeBuildInfo      `yaml:"build,omitempty"`
	OomScoreAdj              *WarningType           `yaml:"oom_score_adj,omitempty"`
	DeviceCgroupRules        *WarningType           `yaml:"device_cgroup_rules,omitempty"`
	Devices                  *WarningType           `yaml:"devices,omitempty"`
	Dns                      *WarningType           `yaml:"dns,omitempty"`
	DnsOpt                   *WarningType           `yaml:"dns_opt,omitempty"`
	DnsSearch                *WarningType           `yaml:"dns_search,omitempty"`
	DomainName               *WarningType           `yaml:"domainname,omitempty"`
	Extends                  *WarningType           `yaml:"extends,omitempty"`
	ExternalLinks            *WarningType           `yaml:"external_links,omitempty"`
	ExtraHosts               *WarningType           `yaml:"extra_hosts,omitempty"`
	GroupAdd                 *WarningType           `yaml:"group_add,omitempty"`
	Hostname                 *WarningType           `yaml:"hostname,omitempty"`
	Init                     *WarningType           `yaml:"init,omitempty"`
	Ipc                      *WarningType           `yaml:"ipc,omitempty"`
	Isolation                *WarningType           `yaml:"isolation,omitempty"`
	Links                    *WarningType           `yaml:"links,omitempty"`
	Logging                  *WarningType           `yaml:"logging,omitempty"`
	Network_mode             *WarningType           `yaml:"network_mode,omitempty"`
	Configs                  *WarningType           `yaml:"configs,omitempty"`
	MacAddress               *WarningType           `yaml:"mac_address,omitempty"`
	Deploy                   *DeployInfoRaw         `yaml:"deploy,omitempty"`
	MemswapLimit             *WarningType           `yaml:"memswap_limit,omitempty"`
	OomKillDisable           *WarningType           `yaml:"oom_kill_disable,omitempty"`
	MemReservation           Quantity               `yaml:"mem_reservation,omitempty"`
	CpuCount                 Quantity               `yaml:"cpu_count,omitempty"`
	Cpus                     Quantity               `yaml:"cpus,omitempty"`
	MemLimit                 Quantity               `yaml:"mem_limit,omitempty"`
	Restart                  string                 `yaml:"restart,omitempty"`
	Image                    string                 `yaml:"image,omitempty"`
	Workdir                  string                 `yaml:"workdir,omitempty"`
	WorkingDirSneakCase      string                 `yaml:"working_dir,omitempty"`
	Command                  CommandStack           `yaml:"command,omitempty"`
	Volumes                  []build.VolumeMounts   `yaml:"volumes,omitempty"`
	CapAddSneakCase          []apiv1.Capability     `yaml:"cap_add,omitempty"`
	EnvFiles                 env.Files              `yaml:"envFile,omitempty"`
	EnvFilesSneakCase        env.Files              `yaml:"env_file,omitempty"`
	Args                     ArgsStack              `yaml:"args,omitempty"`
	Entrypoint               CommandStack           `yaml:"entrypoint,omitempty"`
	Environment              env.Environment        `yaml:"environment,omitempty"`
	Expose                   []PortRaw              `yaml:"expose,omitempty"`
	Ports                    []PortRaw              `yaml:"ports,omitempty"`
	CapDrop                  []apiv1.Capability     `yaml:"capDrop,omitempty"`
	CapDropSneakCase         []apiv1.Capability     `yaml:"cap_drop,omitempty"`
	CapAdd                   []apiv1.Capability     `yaml:"capAdd,omitempty"`
	Public                   bool                   `yaml:"public,omitempty"`
}

type DeployInfoRaw struct {
	Replicas      *int32            `yaml:"replicas,omitempty"`
	Labels        Labels            `yaml:"labels,omitempty"`
	RestartPolicy *RestartPolicyRaw `yaml:"restart_policy,omitempty"`

	EndpointMode   *WarningType `yaml:"endpoint_mode,omitempty"`
	Mode           *WarningType `yaml:"mode,omitempty"`
	Placement      *WarningType `yaml:"placement,omitempty"`
	Constraints    *WarningType `yaml:"constraints,omitempty"`
	Preferences    *WarningType `yaml:"preferences,omitempty"`
	RollbackConfig *WarningType `yaml:"rollback_config,omitempty"`
	UpdateConfig   *WarningType `yaml:"update_config,omitempty"`

	Extensions map[string]interface{} `yaml:",inline" json:"-"`

	Resources ResourcesRaw `yaml:"resources,omitempty"`
}

type RestartPolicyRaw struct {
	Delay       *WarningType           `yaml:"delay,omitempty"`
	Window      *WarningType           `yaml:"window,omitempty"`
	Extensions  map[string]interface{} `yaml:",inline" json:"-"`
	Condition   string                 `yaml:"condition,omitempty"`
	MaxAttempts int32                  `yaml:"max_attempts,omitempty"`
}

type PortRaw struct {
	Extensions    map[string]interface{} `yaml:",inline" json:"-"`
	Protocol      apiv1.Protocol
	ContainerPort int32
	HostPort      int32
	ContainerFrom int32
	ContainerTo   int32
	HostFrom      int32
	HostTo        int32
}

type WarningType struct {
	used bool
}

type ResourcesRaw struct {
	Extensions   map[string]interface{} `yaml:",inline" json:"-"`
	Limits       DeployComposeResources `json:"limits,omitempty" yaml:"limits,omitempty"`
	Reservations DeployComposeResources `json:"reservations,omitempty" yaml:"reservations,omitempty"`
}

type DeployComposeResources struct {
	Devices    *WarningType           `json:"devices,omitempty" yaml:"devices,omitempty"`
	Extensions map[string]interface{} `yaml:",inline" json:"-"`
	Cpus       Quantity               `json:"cpus,omitempty" yaml:"cpus,omitempty"`
	Memory     Quantity               `json:"memory,omitempty" yaml:"memory,omitempty"`
}

type VolumeTopLevel struct {
	Labels      Labels                 `json:"labels,omitempty" yaml:"labels,omitempty"`
	Annotations Annotations            `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	DriverOpts  map[string]string      `json:"driver_opts,omitempty" yaml:"driver_opts,omitempty"`
	Driver      *WarningType           `json:"driver,omitempty" yaml:"driver,omitempty"`
	External    *WarningType           `json:"external,omitempty" yaml:"external,omitempty"`
	Extensions  map[string]interface{} `yaml:",inline" json:"-"`
	Size        Quantity               `json:"size,omitempty" yaml:"size,omitempty"`
	Name        string                 `json:"name,omitempty" yaml:"name,omitempty"`
	Class       string                 `json:"class,omitempty" yaml:"class,omitempty"`
}
type RawMessage struct {
	unmarshal func(interface{}) error
}

type composeBuildInfo struct {
	Name             string               `yaml:"name,omitempty"`
	Context          string               `yaml:"context,omitempty"`
	Dockerfile       string               `yaml:"dockerfile,omitempty"`
	CacheFrom        cache.From           `yaml:"cache_from,omitempty"`
	Target           string               `yaml:"target,omitempty"`
	Args             build.Args           `yaml:"args,omitempty"`
	Image            string               `yaml:"image,omitempty"`
	VolumesToInclude []build.VolumeMounts `yaml:"-"`
	ExportCache      cache.ExportCache    `yaml:"export_cache,omitempty"`
}

func (c *composeBuildInfo) toBuildInfo() *build.Info {
	if c == nil {
		return nil
	}
	return &build.Info{
		Name:             c.Name,
		Context:          c.Context,
		Dockerfile:       c.Dockerfile,
		CacheFrom:        c.CacheFrom,
		Target:           c.Target,
		Args:             c.Args,
		Image:            c.Image,
		VolumesToInclude: c.VolumesToInclude,
		ExportCache:      c.ExportCache,
	}
}

func (c *composeBuildInfo) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rawString string
	err := unmarshal(&rawString)
	if err == nil {
		c.Name = rawString
		return nil
	}

	type composeBuildInfoExtended composeBuildInfo // prevent recursion
	var rawBuildInfo *composeBuildInfoExtended
	err = unmarshal(&rawBuildInfo)
	if err != nil {
		return err
	}
	*c = composeBuildInfo(*rawBuildInfo)
	return nil
}

func (s *Stack) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var stackRaw StackRaw
	err := unmarshal(&stackRaw)
	if err != nil {
		return err
	}

	if err := validateExtensions(stackRaw); err != nil {
		return err
	}
	s.Name = stackRaw.Name

	s.Namespace = stackRaw.Namespace
	s.Context = stackRaw.Context

	s.Endpoints = stackRaw.Endpoints

	s.Volumes = make(map[string]*VolumeSpec)
	for volumeName, volume := range stackRaw.Volumes {
		volumeSpec, err := unmarshalVolume(volume)
		if err != nil {
			return err
		}
		s.Volumes[sanitizeName(volumeName)] = volumeSpec
	}

	sanitizedServicesNames := make(map[string]string)
	s.Services = make(map[string]*Service)
	for svcName, svcRaw := range stackRaw.Services {
		if shouldBeSanitized(svcName) {
			newName := sanitizeName(svcName)
			sanitizedServicesNames[svcName] = newName
			svcName = newName
		}
		s.Services[svcName], err = svcRaw.ToService(svcName, s)
		if err != nil {
			return err
		}
	}

	s.Warnings.NotSupportedFields = getNotSupportedFields(&stackRaw)
	s.Warnings.SanitizedServices = sanitizedServicesNames
	s.Warnings.VolumeMountWarnings = make([]string, 0)
	return nil
}

func unmarshalVolume(volume *VolumeTopLevel) (*VolumeSpec, error) {

	result := &VolumeSpec{}
	if volume == nil {
		result.Labels = make(Labels)
		result.Annotations = make(Annotations)
	} else {
		result.Labels = make(Labels)
		if volume.Annotations == nil {
			result.Annotations = make(Annotations)
		} else {
			result.Annotations = volume.Annotations
		}

		for key, value := range volume.Labels {
			result.Annotations[key] = value
		}

		if volume.Size.Value.Cmp(resource.MustParse("0")) > 0 {
			result.Size = volume.Size
		}
		if volume.DriverOpts != nil {
			for key, value := range volume.DriverOpts {
				if key == "size" {
					qK8s, err := resource.ParseQuantity(value)
					if err != nil {
						return nil, err
					}
					result.Size.Value = qK8s
				}
				if key == "class" {
					result.Class = value
				}
			}
		}
		if result.Class == "" {
			result.Class = volume.Class
		}
	}

	return result, nil

}

func getAccessiblePorts(ports []PortRaw) []PortRaw {
	accessiblePorts := make([]PortRaw, 0)
	for _, p := range ports {
		if p.HostPort != 0 && !IsSkippablePort(p.ContainerPort) {
			accessiblePorts = append(accessiblePorts, p)
		}
	}
	return accessiblePorts
}

func (serviceRaw *ServiceRaw) ToService(svcName string, stack *Stack) (*Service, error) {
	svc := &Service{}
	var err error

	svc.Resources = unmarshalDeployResources(serviceRaw.Deploy, serviceRaw.Resources, serviceRaw.CpuCount, serviceRaw.Cpus, serviceRaw.MemLimit, serviceRaw.MemReservation)

	svc.Replicas = unmarshalDeployReplicas(serviceRaw.Deploy, serviceRaw.Scale, serviceRaw.Replicas)

	svc.Image = serviceRaw.Image
	svc.Build = serviceRaw.Build.toBuildInfo()

	svc.CapAdd = serviceRaw.CapAdd
	if len(serviceRaw.CapAddSneakCase) > 0 {
		svc.CapAdd = serviceRaw.CapAddSneakCase
	}
	svc.CapDrop = serviceRaw.CapDrop
	if len(serviceRaw.CapDropSneakCase) > 0 {
		svc.CapDrop = serviceRaw.CapDropSneakCase
	}

	if err := validateHealthcheck(serviceRaw.Healthcheck); err != nil {
		return nil, err
	}
	if serviceRaw.Healthcheck != nil && !serviceRaw.Healthcheck.Disable {
		svc.Healtcheck = serviceRaw.Healthcheck
		translateHealtcheckCurlToHTTP(svc.Healtcheck)
	}

	svc.Annotations = serviceRaw.Annotations
	if svc.Annotations == nil {
		svc.Annotations = make(Annotations)
	}
	svc.NodeSelector = serviceRaw.NodeSelector

	if stack.IsCompose {
		if len(serviceRaw.Args.Values) > 0 {
			return nil, fmt.Errorf("unsupported field for services.%s: 'args'", svcName)
		}
		svc.Entrypoint.Values = serviceRaw.Entrypoint.Values
		svc.Command.Values = serviceRaw.Command.Values

		for key, value := range unmarshalLabels(serviceRaw.Labels, serviceRaw.Deploy) {
			svc.Annotations[key] = value
		}

	} else { // isOktetoStack
		if len(serviceRaw.Entrypoint.Values) > 0 {
			return nil, fmt.Errorf("unsupported field for services.%s: 'entrypoint'", svcName)
		}
		svc.Entrypoint.Values = serviceRaw.Command.Values
		if len(serviceRaw.Command.Values) == 1 {
			if strings.Contains(serviceRaw.Command.Values[0], " ") {
				svc.Entrypoint.Values = []string{"sh", "-c", serviceRaw.Command.Values[0]}
			}
		}
		svc.Command.Values = serviceRaw.Args.Values

		if svc.Labels == nil {
			svc.Labels = make(Labels)
		}
		for key, value := range unmarshalLabels(serviceRaw.Labels, serviceRaw.Deploy) {
			svc.Labels[key] = value
		}
	}

	svc.EnvFiles = serviceRaw.EnvFiles
	if len(serviceRaw.EnvFilesSneakCase) > 0 {
		svc.EnvFiles = serviceRaw.EnvFilesSneakCase
	}

	svc.Environment = env.Environment{}
	for _, env := range serviceRaw.Environment {
		if env.Value == "" {
			env.Value = os.Getenv(env.Name)
		}
		if env.Value != "" {
			svc.Environment = append(svc.Environment, env)
		}
	}

	svc.DependsOn = make(DependsOn)
	for name, condition := range serviceRaw.DependsOn {
		svc.DependsOn[sanitizeName(name)] = condition
	}

	svc.Public, svc.Ports, err = getSvcPorts(serviceRaw.Public, serviceRaw.Ports, serviceRaw.Expose)
	if err != nil {
		return nil, err
	}

	svc.StopGracePeriod, err = unmarshalDuration(serviceRaw.StopGracePeriod)
	if err != nil {
		return nil, err
	}

	if serviceRaw.StopGracePeriodSneakCase != nil {
		svc.StopGracePeriod, err = unmarshalDuration(serviceRaw.StopGracePeriodSneakCase)
		if err != nil {
			return nil, err
		}
	}

	svc.Volumes, svc.VolumeMounts = splitVolumesByType(serviceRaw.Volumes, stack)
	for idx, volume := range svc.VolumeMounts {
		if !isNamedVolumeDeclared(volume) {
			return nil, fmt.Errorf("named volume '%s' is used in service '%s' but no declaration was found in the volumes section", volume.ToString(), svcName)
		}
		volume.LocalPath, err = filepath.Abs(volume.LocalPath)
		if err != nil {
			return nil, err
		}
		svc.VolumeMounts[idx] = volume
	}

	svc.Workdir = serviceRaw.Workdir
	if serviceRaw.WorkingDirSneakCase != "" {
		svc.Workdir = serviceRaw.WorkingDirSneakCase
	}

	svc.User = serviceRaw.User

	svc.RestartPolicy, err = getRestartPolicy(svcName, serviceRaw.Deploy, serviceRaw.Restart)
	if err != nil {
		return nil, err
	}
	if serviceRaw.Deploy != nil && serviceRaw.Deploy.RestartPolicy != nil {
		svc.BackOffLimit = serviceRaw.Deploy.RestartPolicy.MaxAttempts
	}
	return svc, nil
}

type healthCheckunmarshaller struct {
	HTTP        *HTTPHealtcheck `yaml:"http,omitempty"`
	Readiness   *bool           `yaml:"x-okteto-readiness,omitempty"`
	Test        HealtcheckTest  `yaml:"test,omitempty"`
	Interval    time.Duration   `yaml:"interval,omitempty"`
	Timeout     time.Duration   `yaml:"timeout,omitempty"`
	StartPeriod time.Duration   `yaml:"start_period,omitempty"`
	Retries     int             `yaml:"retries,omitempty"`
	Disable     bool            `yaml:"disable,omitempty"`
	Liveness    bool            `yaml:"x-okteto-liveness,omitempty"`
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (hc *HealthCheck) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rawHealthcheck healthCheckunmarshaller
	err := unmarshal(&rawHealthcheck)
	if err != nil {
		return fmt.Errorf("error reading healthchecks: %w", err)
	}

	var readiness bool
	if rawHealthcheck.Readiness == nil {
		readiness = true
	} else {
		readiness = *rawHealthcheck.Readiness
	}

	if !rawHealthcheck.Liveness && !readiness {
		return fmt.Errorf("error reading healthchecks: 'x-okteto-liveness' or 'x-okteto-readiness' must be true")
	}

	*hc = HealthCheck{
		HTTP:        rawHealthcheck.HTTP,
		Test:        rawHealthcheck.Test,
		Interval:    rawHealthcheck.Interval,
		Timeout:     rawHealthcheck.Timeout,
		Retries:     rawHealthcheck.Retries,
		StartPeriod: rawHealthcheck.StartPeriod,
		Disable:     rawHealthcheck.Disable,
		Liveness:    rawHealthcheck.Liveness,
		Readiness:   readiness,
	}
	return nil
}

func validateHealthcheck(healthcheck *HealthCheck) error {
	if healthcheck != nil && len(healthcheck.Test) != 0 && healthcheck.Test[0] == "NONE" {
		healthcheck.Test = make(HealtcheckTest, 0)
		healthcheck.Disable = true
	}
	if healthcheck != nil && healthcheck.HTTP == nil && len(healthcheck.Test) == 0 && !healthcheck.Disable {
		return fmt.Errorf("Healthcheck.test must be set")
	}
	if healthcheck != nil && healthcheck.HTTP != nil && len(healthcheck.Test) != 0 && !healthcheck.Disable {
		return fmt.Errorf("healthcheck.test can not be set along with healthcheck.http")
	}
	return nil
}

func translateHealtcheckCurlToHTTP(healthcheck *HealthCheck) {
	// Join and then split the strings by space to ensure that
	// each element in the string slice is a contiguous string with
	// no spaces.
	s := strings.Join(healthcheck.Test, " ")
	testStrings := strings.Split(s, " ")

	minTestStrings := 2
	// There should be at least two strings, the curl binary and url.
	if len(testStrings) < minTestStrings {
		return
	}

	if testStrings[0] != "curl" {
		return
	}

	var checkURL *url.URL
	for _, possibleURL := range testStrings[1:] {
		if possibleURL == "-f" || possibleURL == "--fail" {
			continue
		}

		u, err := url.ParseRequestURI(possibleURL)
		// It's possible to have a healthcheck url without the scheme. If
		// that happens then we inject one and attempt parsing again.
		if err != nil || u.Host == "" {
			u, err = url.ParseRequestURI("https://" + possibleURL)
			if err != nil {
				u = nil
				continue
			}
		}
		checkURL = u
		break
	}

	if checkURL == nil {
		return
	}

	p := checkURL.Port()
	if p == "" {
		return
	}
	port, err := strconv.ParseInt(p, 10, 32)
	if err != nil {
		return
	}

	var path string
	if checkURL.Path == "" {
		path = "/"
	} else {
		path = checkURL.Path
	}

	healthcheck.HTTP = &HTTPHealtcheck{Path: path, Port: int32(port)}
	healthcheck.Test = make(HealtcheckTest, 0)
}

func getSvcPorts(public bool, rawPorts, rawExpose []PortRaw) (bool, []Port, error) {
	rawPorts = expandRangePorts(rawPorts)

	// If public is set at this point it means that it comes from an okteto stacks and it's true
	if public {
		rawPorts = translateOktetoStacksPortsIntoComposeSyntax(rawPorts)
	}

	if !public && len(getAccessiblePorts(rawPorts)) == 1 {
		public = true
	}
	if len(rawExpose) > 0 && len(rawPorts) == 0 {
		public = false
	}

	ports := make([]Port, 0)
	for _, p := range rawPorts {
		if err := validatePort(p, ports); err == nil {
			ports = append(ports, Port{HostPort: p.HostPort, ContainerPort: p.ContainerPort, Protocol: p.Protocol})
		} else {
			return false, ports, err
		}
	}
	rawExpose = expandRangePorts(rawExpose)
	for _, p := range rawExpose {
		newPort := Port{HostPort: p.HostPort, ContainerPort: p.ContainerPort, Protocol: p.Protocol}
		if p.ContainerPort == 0 {
			if !IsAlreadyAdded(newPort, ports) {
				ports = append(ports, newPort)
			}
		} else {
			if !IsAlreadyAddedExpose(newPort, ports) {
				ports = append(ports, newPort)
			}
		}
	}
	return public, ports, nil
}

// translateOktetoStacksPortsIntoComposeSyntax translates okteto stack port syntax into compose ports syntax
// We need this translation because we are using the same serializer for okteto stacks and docker compose syntax
// Otherwise we don't know which ports should create an endpoint and which ones not
// Example 1:
// Stacks syntax  -> Compose syntax
// ports:         | ports:
//   - 5000       |   - 5000:5000
//   - 3000       |   - 3000:3000
//
// public: true   |
func translateOktetoStacksPortsIntoComposeSyntax(ports []PortRaw) []PortRaw {
	for idx, p := range ports {
		if p.HostPort == 0 {
			p.HostPort = p.ContainerPort
		}
		ports[idx] = p
	}
	return ports
}

func expandRangePorts(ports []PortRaw) []PortRaw {
	newPortList := make([]PortRaw, 0)
	for _, p := range ports {
		if p.ContainerPort != 0 {
			newPortList = append(newPortList, p)
		} else {
			portStart := p.ContainerFrom
			portFinish := p.ContainerTo
			aux := 0
			for portStart+int32(aux) != portFinish+1 {
				if p.HostFrom != 0 {
					newPortList = append(newPortList, PortRaw{ContainerPort: p.ContainerFrom + int32(aux)})
				} else {
					newPortList = append(newPortList, PortRaw{ContainerPort: p.ContainerFrom + int32(aux), HostPort: 0})
				}
				if portStart > portFinish {
					aux--
				} else {
					aux++
				}

			}
		}
	}
	return newPortList
}

func validatePort(newPort PortRaw, ports []Port) error {
	for _, p := range ports {
		if newPort.ContainerPort == p.HostPort {
			return fmt.Errorf("container port '%d' is already declared as host port in port '%d:%d'", newPort.ContainerPort, p.HostPort, p.ContainerPort)
		}
		if newPort.HostPort == p.ContainerPort {
			if p.HostPort == 0 {
				return fmt.Errorf("host port '%d' is already declared as container port in port '%d'", newPort.HostPort, p.ContainerPort)
			} else {
				return fmt.Errorf("host port '%d' is already declared as container port in port '%d:%d'", newPort.HostPort, p.HostPort, p.ContainerPort)
			}
		}
	}
	return nil
}

func isNamedVolumeDeclared(volume build.VolumeMounts) bool {
	if volume.LocalPath != "" {
		wd, err := os.Getwd()
		if err != nil {
			return false
		}
		relative := true
		if filepath.IsAbs(volume.LocalPath) {
			_, err = filepath.Rel(wd, volume.LocalPath)
			relative = err == nil
		}

		if strings.HasPrefix(volume.LocalPath, "/") && relative {
			return true
		}
		if strings.HasPrefix(volume.LocalPath, "./") && relative {
			return true
		}
		if filesystem.FileExists(volume.LocalPath) && relative {
			return true
		}
	}
	return false
}

func splitVolumesByType(volumes []build.VolumeMounts, s *Stack) ([]build.VolumeMounts, []build.VolumeMounts) {
	topLevelVolumes := make([]build.VolumeMounts, 0)
	mountedVolumes := make([]build.VolumeMounts, 0)
	for _, volume := range volumes {
		if volume.LocalPath == "" || isInVolumesTopLevelSection(sanitizeName(volume.LocalPath), s) {
			volume.LocalPath = sanitizeName(volume.LocalPath)
			topLevelVolumes = append(topLevelVolumes, volume)
		} else {
			mountedVolumes = append(mountedVolumes, volume)
		}
	}
	return topLevelVolumes, mountedVolumes
}

func unmarshalLabels(labels Labels, deployInfo *DeployInfoRaw) Labels {
	result := Labels{}
	if deployInfo != nil {
		if deployInfo.Labels != nil {
			for key, value := range deployInfo.Labels {
				result[key] = value
			}
		}
	}
	for key, value := range labels {
		result[key] = value
	}
	return result
}

func (msg *RawMessage) UnmarshalYAML(unmarshal func(interface{}) error) error {
	msg.unmarshal = unmarshal
	return nil
}

func (msg *RawMessage) Unmarshal(v interface{}) error {
	return msg.unmarshal(v)
}

func (warning *WarningType) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var input interface{}
	a := unmarshal(&input)
	if a != nil {
		warning.used = true
	}
	return nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (sc *StackSecurityContext) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rawSecurityContext string
	err := unmarshal(&rawSecurityContext)
	if err == nil {
		if strings.Contains(rawSecurityContext, ":") {
			securityContextLength := 2
			parts := strings.Split(rawSecurityContext, ":")
			if len(parts) != securityContextLength {
				return fmt.Errorf("securityContext '%s' is malformed. Only 'dddd:dddd' is supported", rawSecurityContext)
			}
			runAsUser, err := strconv.ParseInt(parts[0], 10, 64)
			if err != nil {
				return fmt.Errorf("cannot obtain UID from '%s'", parts[0])
			}
			runAsGroup, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				return fmt.Errorf("cannot obtain GID from '%s'", parts[1])
			}
			sc.RunAsUser = &runAsUser
			sc.RunAsGroup = &runAsGroup
		} else {
			runAsUser, err := strconv.ParseInt(rawSecurityContext, 10, 64)
			if err != nil {
				return fmt.Errorf("cannot obtain UID from '%s'", rawSecurityContext)
			}
			sc.RunAsUser = &runAsUser
		}
		return nil
	}
	type stackSecurityContext StackSecurityContext // prevent recursion
	var expandedSecurityContext stackSecurityContext
	err = unmarshal(&expandedSecurityContext)
	if err != nil {
		return err
	}
	sc.RunAsUser = expandedSecurityContext.RunAsUser
	sc.RunAsGroup = expandedSecurityContext.RunAsGroup
	return nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (dependsOn *DependsOn) UnmarshalYAML(unmarshal func(interface{}) error) error {
	result := make(DependsOn)

	type dependsOnSyntax DependsOn // prevent recursion
	var d dependsOnSyntax
	err := unmarshal(&d)
	if err == nil {
		for key, value := range d {
			if value.Condition != DependsOnServiceRunning && value.Condition != DependsOnServiceHealthy && value.Condition != DependsOnServiceCompleted {
				return fmt.Errorf("'%s' is unsupported. Condition must be one of '%s', '%s' or '%s'", value.Condition, DependsOnServiceRunning, DependsOnServiceHealthy, DependsOnServiceCompleted)
			}
			result[key] = value
		}
		*dependsOn = result
		return nil
	}
	var dList []string
	err = unmarshal(&dList)
	if err == nil {
		for _, svc := range dList {
			result[svc] = DependsOnConditionSpec{Condition: DependsOnServiceRunning}
		}
		*dependsOn = result
		return nil
	}
	return err
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (p *PortRaw) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rawPortString string
	err := unmarshal(&rawPortString)
	if err != nil {
		return fmt.Errorf("Port field is only supported in short syntax")
	}

	if !strings.Contains(rawPortString, ":") {
		err = getPortWithoutMapping(p, rawPortString)
	} else {
		err = getPortWithMapping(p, rawPortString)
	}

	if err != nil {
		return err
	}
	return nil
}

func getPortWithoutMapping(p *PortRaw, portString string) error {
	var err error
	p.ContainerFrom, p.ContainerTo, p.Protocol, err = getRangePorts(portString)
	if err != nil {
		return err
	}
	if p.ContainerFrom == 0 {
		portDigit, protocol, err := getPortAndProtocol(portString, portString)
		if err != nil {
			return err
		}
		port, err := getPortFromString(portDigit, portString)
		if err != nil {
			return err
		}
		p.ContainerPort = port
		p.Protocol = protocol
	}
	return nil
}

func getRangePorts(portString string) (int32, int32, apiv1.Protocol, error) {
	if strings.Contains(portString, "-") {
		rangePortsLength := 2
		rangeSplitted := strings.Split(portString, "-")
		if len(rangeSplitted) != rangePortsLength {
			return 0, 0, "", fmt.Errorf("cannot convert '%s' to a port.", portString)
		}
		fromString, toString := rangeSplitted[0], rangeSplitted[1]
		toString, protocol, err := getPortAndProtocol(toString, portString)
		if err != nil {
			return 0, 0, "", err
		}
		fromPort, err := getPortFromString(fromString, portString)
		if err != nil {
			return 0, 0, "", err
		}
		toPort, err := getPortFromString(toString, portString)
		if err != nil {
			return 0, 0, "", err
		}
		return fromPort, toPort, protocol, nil
	}
	return 0, 0, "", nil
}

func getPortFromString(portString, originalPortString string) (int32, error) {
	port, err := strconv.ParseInt(portString, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("cannot convert '%s' to a port: %s is not a number", originalPortString, portString)
	}
	return int32(port), nil
}

func getPortAndProtocol(portString, originalPortString string) (string, apiv1.Protocol, error) {
	var err error
	protocol := apiv1.ProtocolTCP
	if strings.Contains(portString, "/") {
		protocolLength := 2
		portProtocolSplitted := strings.Split(portString, "/")
		if len(portProtocolSplitted) != protocolLength {
			return "", protocol, fmt.Errorf("cannot convert '%s' to a port", originalPortString)
		}
		portString = portProtocolSplitted[0]
		protocol, err = getProtocol(portProtocolSplitted[1])
		if err != nil {
			return "", protocol, fmt.Errorf("cannot convert '%s' to a port: %w", originalPortString, err)
		}
	}
	return portString, protocol, nil
}

func getPortWithMapping(p *PortRaw, portString string) error {
	localToContainer := strings.Split(portString, ":")
	maxLength := 3
	if len(localToContainer) > maxLength {
		return fmt.Errorf(forward.MalformedPortForward, portString)
	}

	containerPortString := localToContainer[len(localToContainer)-1]
	var err error
	p.ContainerFrom, p.ContainerTo, p.Protocol, err = getRangePorts(containerPortString)
	if err != nil {
		return err
	}
	hostPortString := strings.Join(localToContainer[:len(localToContainer)-1], ":")
	p.HostFrom, p.HostTo, _, err = getRangePorts(hostPortString)
	if err != nil {
		return err
	}
	if (p.ContainerFrom - p.ContainerTo) != (p.HostFrom - p.HostTo) {
		return fmt.Errorf("cannot convert '%s' to a port: Ranges must be of the same length", portString)
	}

	if p.ContainerFrom == 0 {
		if strings.Contains(hostPortString, ":") {
			return fmt.Errorf("cannot convert '%s' to a port: Host IP is not allowed", portString)
		}

		p.HostPort, err = getPortFromString(hostPortString, portString)
		if err != nil {
			return err
		}

		portDigit, protocol, err := getPortAndProtocol(containerPortString, portString)
		if err != nil {
			return err
		}
		port, err := getPortFromString(portDigit, portString)
		if err != nil {
			return err
		}
		p.ContainerPort = port
		p.Protocol = protocol
	}
	return nil
}

func IsSkippablePort(port int32) bool {
	skippablePorts := map[int32]string{3306: "MySQL", 1521: "OracleDB", 1830: "OracleDB", 5432: "PostgreSQL",
		1433: "SQL Server", 1434: "SQL Server", 7210: "MaxDB", 7473: "Neo4j", 7474: "Neo4j", 8529: "ArangoDB",
		7000: "Cassandra", 7001: "Cassandra", 9042: "Cassandra", 8086: "InfluxDB", 9200: "Elasticsearch", 9300: "Elasticsearch",
		5984: "CouchDB", 27017: "MongoDB", 27018: "MongoDB", 27019: "MongoDB", 28017: "MongoDB", 6379: "Redis",
		8087: "Riak", 8098: "Riak", 828015: "Rethink", 29015: "Rethink", 28015: "Rethink", 7574: "Solr", 8983: "Solr",
		2345: "Golang debugger", 5858: "Node debugger", 9229: "Node debugger", 5005: "Java debugger", 1234: "Ruby debugger",
		4444: "Python pdb", 5678: "Python debugpy"}
	if _, ok := skippablePorts[port]; ok {
		return true
	}
	return false
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (p *Port) MarshalYAML() (interface{}, error) {
	return Port{ContainerPort: p.ContainerPort, Protocol: p.Protocol}, nil
}

func getRestartPolicy(svcName string, deployInfo *DeployInfoRaw, restartPolicy string) (apiv1.RestartPolicy, error) {
	var restart string
	if deployInfo != nil && deployInfo.RestartPolicy != nil {
		restart = deployInfo.RestartPolicy.Condition
	}
	if restart == "" {
		restart = restartPolicy
	}
	switch restart {
	case "none", "never", "no":
		return apiv1.RestartPolicyNever, nil
	case "always", "", "unless-stopped", "any":
		return apiv1.RestartPolicyAlways, nil
	case "on-failure":
		return apiv1.RestartPolicyOnFailure, nil
	default:
		return apiv1.RestartPolicyAlways, fmt.Errorf("cannot create container for service %s: invalid restart policy '%s'", svcName, restart)
	}
}
func unmarshalDeployResources(deployInfo *DeployInfoRaw, resources *StackResources, cpuCount, cpus, memLimit, memReservation Quantity) *StackResources {
	if resources == nil {
		resources = &StackResources{}
	}
	if deployInfo != nil {
		resources.Limits = deployInfo.Resources.Limits.toServiceResources()
		resources.Requests = deployInfo.Resources.Reservations.toServiceResources()
	}

	if resources.Limits.CPU.Value.IsZero() && !cpuCount.Value.IsZero() {
		resources.Limits.CPU = cpuCount
	}

	if resources.Requests.CPU.Value.IsZero() && !cpus.Value.IsZero() {
		resources.Requests.CPU = cpus
	}

	if resources.Limits.Memory.Value.IsZero() && !memLimit.Value.IsZero() {
		resources.Limits.Memory = memLimit
	}

	if resources.Requests.Memory.Value.IsZero() && !memReservation.Value.IsZero() {
		resources.Requests.Memory = memReservation
	}

	return resources
}

func unmarshalDeployReplicas(deployInfo *DeployInfoRaw, scale, replicas *int32) int32 {
	if replicas != nil {
		return *replicas
	}
	if deployInfo != nil && deployInfo.Replicas != nil {
		return *deployInfo.Replicas
	}
	if scale != nil {
		return *scale
	}
	return DefaultReplicasNumber
}

func (r DeployComposeResources) toServiceResources() ServiceResources {
	resources := ServiceResources{}
	if !r.Cpus.Value.IsZero() {
		resources.CPU = r.Cpus
	}
	if !r.Memory.Value.IsZero() {
		resources.Memory = r.Memory
	}
	return resources
}

func (endpoint *EndpointSpec) UnmarshalYAML(unmarshal func(interface{}) error) error {
	result := make(EndpointSpec)
	if endpoint == nil {
		*endpoint = result
	}
	var directRule Endpoint
	err := unmarshal(&directRule)
	if err == nil {
		result[""] = directRule
		*endpoint = result
		return nil
	}
	var expandedAnnotation map[string]Endpoint
	err = unmarshal(&expandedAnnotation)
	if err != nil {
		return err
	}

	*endpoint = expandedAnnotation
	return nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (s *StackResources) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type stackResources StackResources // prevent recursion
	var r stackResources
	err := unmarshal(&r)
	if err == nil {
		s.Limits = r.Limits
		s.Requests = r.Requests
		return nil
	}

	var resources ServiceResources
	err = unmarshal(&resources)
	if err != nil {
		return err
	}
	s.Limits.CPU = resources.CPU
	s.Limits.Memory = resources.Memory
	s.Requests.Storage = resources.Storage
	return nil
}

func unmarshalDuration(raw *RawMessage) (int64, error) {
	var duration int64
	if raw == nil {
		return duration, nil
	}

	var durationString string
	err := raw.unmarshal(&durationString)
	if err != nil {
		return duration, err
	}
	seconds, err := strconv.Atoi(durationString)
	if err != nil {
		var d time.Duration
		err = raw.unmarshal(&d)
		if err != nil {
			return duration, err
		}
		return int64(d.Seconds()), nil
	}
	return int64(seconds), nil

}
func (httpHealtcheck *HTTPHealtcheck) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type httpHealtCheck HTTPHealtcheck // prevent recursion
	var healthcheck httpHealtCheck
	err := unmarshal(&healthcheck)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(healthcheck.Path, "/") {
		return fmt.Errorf("HTTP path must start with '/'")
	}

	httpHealtcheck.Path = healthcheck.Path
	httpHealtcheck.Port = healthcheck.Port
	return nil
}
func (healthcheckTest *HealtcheckTest) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rawList []string
	err := unmarshal(&rawList)

	healthCheckLength := 2
	if err == nil {
		if len(rawList) == 0 {
			return fmt.Errorf("healtcheck.test can not be an empty list")
		}
		switch rawList[0] {
		case "NONE":
			*healthcheckTest = rawList[:1]
			return nil
		case "CMD":
			*healthcheckTest = rawList[1:]
		case "CMD-SHELL":
			if len(rawList) != healthCheckLength {
				return fmt.Errorf("'CMD-SHELL' healtcheck.test must have exactly 2 elements")
			}
			*healthcheckTest, err = shellquote.Split(rawList[1])
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("when 'healtcheck.test' is a list the first item must be either 'NONE', 'CMD' or 'CMD-SHELL'")
		}
		return nil
	}

	var rawString string
	err = unmarshal(&rawString)
	if err != nil {
		return err
	}
	*healthcheckTest, err = shellquote.Split(rawString)
	if err != nil {
		return err
	}
	return nil
}

func getProtocol(protocolName string) (apiv1.Protocol, error) {
	protocolName = strings.ToLower(protocolName)
	switch protocolName {
	case "tcp":
		return apiv1.ProtocolTCP, nil
	case "udp":
		return apiv1.ProtocolUDP, nil
	case "sctp":
		return apiv1.ProtocolSCTP, nil
	default:
		return apiv1.ProtocolTCP, fmt.Errorf("%s is not supported as a protocol", protocolName)
	}
}

func hasUppercase(name string) bool {
	for _, s := range name {
		if unicode.IsUpper(s) {
			return true
		}
	}
	return false
}

func hasEmptySpace(name string) bool {
	return strings.Contains(name, " ")
}

func hasUnderscore(name string) bool {
	return strings.Contains(name, "_")
}

func shouldBeSanitized(name string) bool {
	return hasEmptySpace(name) || hasUnderscore(name) || hasUppercase(name)
}

func sanitizeName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.ToLower(name)
	return name
}

func getNotSupportedFields(s *StackRaw) []string {
	notSupportedFields := make([]string, 0)
	notSupportedFields = append(notSupportedFields, getTopLevelNotSupportedFields(s)...)
	for name, svcInfo := range s.Services {
		notSupportedFields = append(notSupportedFields, getServiceNotSupportedFields(name, svcInfo)...)
	}
	for name, volumeInfo := range s.Volumes {
		if volumeInfo != nil {
			notSupportedFields = append(notSupportedFields, getVolumesNotSupportedFields(name, volumeInfo)...)
		}
	}
	return notSupportedFields
}

func getTopLevelNotSupportedFields(s *StackRaw) []string {
	notSupported := make([]string, 0)
	if s.Networks != nil {
		notSupported = append(notSupported, "networks")
	}
	if s.Configs != nil {
		notSupported = append(notSupported, "configs")
	}
	if s.Secrets != nil {
		notSupported = append(notSupported, "secrets")
	}
	return notSupported
}

func getServiceNotSupportedFields(svcName string, svcInfo *ServiceRaw) []string {
	notSupported := make([]string, 0)
	if svcInfo.Deploy != nil {
		notSupported = append(notSupported, getDeployNotSupportedFields(svcName, svcInfo.Deploy)...)
	}

	if svcInfo.BlkioConfig != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].blkio_config", svcName))
	}
	if svcInfo.CpuPercent != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].cpu_percent", svcName))
	}
	if svcInfo.CpuShares != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].cpu_shares", svcName))
	}
	if svcInfo.CpuPeriod != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].cpu_period", svcName))
	}
	if svcInfo.CpuQuota != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].cpu_quota", svcName))
	}
	if svcInfo.CpuRtRuntime != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].cpu_rt_runtime", svcName))
	}
	if svcInfo.CpuRtPeriod != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].cpu_rt_period", svcName))
	}
	if svcInfo.Cpuset != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].cpuset", svcName))
	}
	if svcInfo.CgroupParent != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].cgroup_parent", svcName))
	}
	if svcInfo.Configs != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].configs", svcName))
	}
	if svcInfo.CredentialSpec != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].credential_spec", svcName))
	}
	if svcInfo.DeviceCgroupRules != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].device_cgroup_rules", svcName))
	}
	if svcInfo.Devices != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].devices", svcName))
	}
	if svcInfo.Dns != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].dns", svcName))
	}
	if svcInfo.DnsOpt != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].dns_opt", svcName))
	}
	if svcInfo.DnsSearch != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].dns_search", svcName))
	}
	if svcInfo.DomainName != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].domainname", svcName))
	}
	if svcInfo.Extends != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].extends", svcName))
	}
	if svcInfo.ExternalLinks != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].external_links", svcName))
	}
	if svcInfo.ExtraHosts != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].extra_hosts", svcName))
	}
	if svcInfo.GroupAdd != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].group_add", svcName))
	}
	if svcInfo.Hostname != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].hostname", svcName))
	}
	if svcInfo.Init != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].init", svcName))
	}
	if svcInfo.Ipc != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].ipc", svcName))
	}
	if svcInfo.Isolation != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].isolation", svcName))
	}
	if svcInfo.Links != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].links", svcName))
	}
	if svcInfo.Logging != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].logging", svcName))
	}
	if svcInfo.Network_mode != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].network_mode", svcName))
	}
	if svcInfo.Networks != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].networks", svcName))
	}
	if svcInfo.MacAddress != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].mac_address", svcName))
	}
	if svcInfo.MemSwappiness != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].mem_swappiness", svcName))
	}
	if svcInfo.MemswapLimit != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].memswap_limit", svcName))
	}
	if svcInfo.OomKillDisable != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].oom_kill_disable", svcName))
	}
	if svcInfo.OomScoreAdj != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].oom_score_adj", svcName))
	}
	if svcInfo.Pid != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].pid", svcName))
	}
	if svcInfo.PidLimit != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].pid_limit", svcName))
	}
	if svcInfo.Platform != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].platform", svcName))
	}
	if svcInfo.Privileged != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].privileged", svcName))
	}
	if svcInfo.Profiles != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].profiles", svcName))
	}
	if svcInfo.PullPolicy != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].pull_policy", svcName))
	}
	if svcInfo.ReadOnly != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].read_only", svcName))
	}
	if svcInfo.Runtime != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].runtime", svcName))
	}
	if svcInfo.Secrets != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].secrets", svcName))
	}
	if svcInfo.SecurityOpt != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].security_opt", svcName))
	}
	if svcInfo.ShmSize != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].shm_size", svcName))
	}
	if svcInfo.StdinOpen != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].stdin_open", svcName))
	}
	if svcInfo.StopSignal != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].stop_signal", svcName))
	}
	if svcInfo.StorageOpts != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].storage_opts", svcName))
	}
	if svcInfo.Sysctls != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].sysctls", svcName))
	}
	if svcInfo.Tmpfs != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].tmpfs", svcName))
	}
	if svcInfo.Tty != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].tty", svcName))
	}
	if svcInfo.Ulimits != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].ulimits", svcName))
	}
	if svcInfo.UsernsMode != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].userns_mode", svcName))
	}
	if svcInfo.VolumesFrom != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].volumes_from", svcName))
	}
	return notSupported
}

func getDeployNotSupportedFields(svcName string, deploy *DeployInfoRaw) []string {
	notSupported := make([]string, 0)

	if deploy.Resources.Limits.Devices != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].deploy.resources.limits.devices", svcName))
	}
	if deploy.Resources.Reservations.Devices != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].deploy.resources.reservations.devices", svcName))
	}

	if deploy.RestartPolicy != nil {
		if deploy.RestartPolicy.Delay != nil {
			notSupported = append(notSupported, fmt.Sprintf("services[%s].deploy.delay", svcName))
		}
		if deploy.RestartPolicy.Window != nil {
			notSupported = append(notSupported, fmt.Sprintf("services[%s].deploy.window", svcName))
		}
	}
	if deploy.EndpointMode != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].deploy.endpoint_mode", svcName))
	}
	if deploy.Mode != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].deploy.mode", svcName))
	}
	if deploy.Placement != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].deploy.placement", svcName))
	}
	if deploy.Constraints != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].deploy.constraints", svcName))
	}
	if deploy.Preferences != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].deploy.preferences", svcName))
	}
	if deploy.RollbackConfig != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].deploy.rollback_config", svcName))
	}
	if deploy.UpdateConfig != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].deploy.update_config", svcName))
	}

	return notSupported
}

func getVolumesNotSupportedFields(volumeName string, volumeInfo *VolumeTopLevel) []string {
	notSupported := make([]string, 0)

	if volumeInfo.Driver != nil {
		notSupported = append(notSupported, fmt.Sprintf("volumes[%s].driver", volumeName))
	}
	if volumeInfo.DriverOpts != nil {
		for key := range volumeInfo.DriverOpts {
			if key != "size" && key != "class" {
				notSupported = append(notSupported, fmt.Sprintf("volumes[%s].driver_opts.%s", volumeName, key))
			}
		}
	}

	if volumeInfo.External != nil {
		notSupported = append(notSupported, fmt.Sprintf("volumes[%s].external", volumeName))
	}
	return notSupported

}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (c *CommandStack) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var multi []string
	err := unmarshal(&multi)
	if err != nil {
		var single string
		err := unmarshal(&single)
		if err != nil {
			return err
		}
		if strings.Contains(single, " && ") {
			c.Values = []string{"sh", "-c", single}
		} else {
			c.Values, err = shellquote.Split(single)
			if err != nil {
				return err
			}
		}
	} else {
		c.Values = multi
	}
	return nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (a *ArgsStack) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var multi []string
	err := unmarshal(&multi)
	if err != nil {
		var single string
		err := unmarshal(&single)
		if err != nil {
			return err
		}
		a.Values, err = shellquote.Split(single)
		if err != nil {
			return err
		}
	} else {
		a.Values = multi
	}
	return nil
}

func validateExtensions(stack StackRaw) error {
	nonValidFields := make([]string, 0)
	for extension := range stack.Extensions {
		if !strings.HasPrefix(extension, "x-") {
			nonValidFields = append(nonValidFields, extension)
		}
	}

	for svcName, svc := range stack.Services {
		if svc == nil {
			return fmt.Errorf("%w: %w", oktetoErrors.ErrInvalidManifest, oktetoErrors.ErrServiceEmpty)
		}
		for extension := range svc.Extensions {
			nonValidFields = append(nonValidFields, fmt.Sprintf("services[%s].%s", svcName, extension))
		}
		if svc.Deploy != nil {
			for extension := range svc.Deploy.Extensions {
				nonValidFields = append(nonValidFields, fmt.Sprintf("services[%s].deploy.%s", svcName, extension))
			}
		}
	}
	if len(nonValidFields) == 1 {
		return fmt.Errorf("invalid compose manifest: Field '%s' is not supported.\n    More information is available here: https://okteto.com/docs/reference/compose/", nonValidFields[0])
	} else if len(nonValidFields) > 1 {
		return fmt.Errorf(`invalid compose manifest: The following fields are not supported.
    - %s
    More information is available here: https://okteto.com/docs/reference/compose/`, strings.Join(nonValidFields, "\n    - "))
	}
	return nil
}
