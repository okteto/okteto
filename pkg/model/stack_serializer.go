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
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kballard/go-shellquote"
	apiv1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
)

//Stack represents an okteto stack
type StackRaw struct {
	Version   string                     `yaml:"version,omitempty"`
	Name      string                     `yaml:"name"`
	Namespace string                     `yaml:"namespace,omitempty"`
	Services  map[string]*ServiceRaw     `yaml:"services,omitempty"`
	Endpoints EndpointSpec               `yaml:"endpoints,omitempty"`
	Volumes   map[string]*VolumeTopLevel `yaml:"volumes,omitempty"`

	// Docker-compose not implemented
	Networks *WarningType `yaml:"networks,omitempty"`

	Configs *WarningType `yaml:"configs,omitempty"`
	Secrets *WarningType `yaml:"secrets,omitempty"`

	Warnings StackWarnings
}

//Service represents an okteto stack service
type ServiceRaw struct {
	Deploy                   *DeployInfoRaw     `yaml:"deploy,omitempty"`
	Build                    *BuildInfo         `yaml:"build,omitempty"`
	CapAddSneakCase          []apiv1.Capability `yaml:"cap_add,omitempty"`
	CapAdd                   []apiv1.Capability `yaml:"capAdd,omitempty"`
	CapDropSneakCase         []apiv1.Capability `yaml:"cap_drop,omitempty"`
	CapDrop                  []apiv1.Capability `yaml:"capDrop,omitempty"`
	Command                  CommandStack       `yaml:"command,omitempty"`
	CpuCount                 Quantity           `yaml:"cpu_count,omitempty"`
	Cpus                     Quantity           `yaml:"cpus,omitempty"`
	Entrypoint               CommandStack       `yaml:"entrypoint,omitempty"`
	Args                     ArgsStack          `yaml:"args,omitempty"`
	EnvFilesSneakCase        EnvFiles           `yaml:"env_file,omitempty"`
	EnvFiles                 EnvFiles           `yaml:"envFile,omitempty"`
	Environment              Environment        `yaml:"environment,omitempty"`
	Expose                   *RawMessage        `yaml:"expose,omitempty"`
	Image                    string             `yaml:"image,omitempty"`
	Labels                   Labels             `json:"labels,omitempty" yaml:"labels,omitempty"`
	Annotations              Annotations        `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	MemLimit                 Quantity           `yaml:"mem_limit,omitempty"`
	MemReservation           Quantity           `yaml:"mem_reservation,omitempty"`
	Ports                    []PortRaw          `yaml:"ports,omitempty"`
	Scale                    int32              `yaml:"scale"`
	StopGracePeriodSneakCase *RawMessage        `yaml:"stop_grace_period,omitempty"`
	StopGracePeriod          *RawMessage        `yaml:"stopGracePeriod,omitempty"`
	Volumes                  []StackVolume      `yaml:"volumes,omitempty"`
	WorkingDirSneakCase      string             `yaml:"working_dir,omitempty"`
	Workdir                  string             `yaml:"workdir,omitempty"`

	Public    bool            `yaml:"public,omitempty"`
	Replicas  int32           `yaml:"replicas"`
	Resources *StackResources `yaml:"resources,omitempty"`

	BlkioConfig       *WarningType `yaml:"blkio_config,omitempty"`
	CpuPercent        *WarningType `yaml:"cpu_percent,omitempty"`
	CpuShares         *WarningType `yaml:"cpu_shares,omitempty"`
	CpuPeriod         *WarningType `yaml:"cpu_period,omitempty"`
	CpuQuota          *WarningType `yaml:"cpu_quota,omitempty"`
	CpuRtRuntime      *WarningType `yaml:"cpu_rt_runtime,omitempty"`
	CpuRtPeriod       *WarningType `yaml:"cpu_rt_period,omitempty"`
	Cpuset            *WarningType `yaml:"cpuset,omitempty"`
	CgroupParent      *WarningType `yaml:"cgroup_parent,omitempty"`
	Configs           *WarningType `yaml:"configs,omitempty"`
	ContainerName     *WarningType `yaml:"container_name,omitempty"`
	CredentialSpec    *WarningType `yaml:"credential_spec,omitempty"`
	DependsOn         *WarningType `yaml:"depends_on,omitempty"`
	DeviceCgroupRules *WarningType `yaml:"device_cgroup_rules,omitempty"`
	Devices           *WarningType `yaml:"devices,omitempty"`
	Dns               *WarningType `yaml:"dns,omitempty"`
	DnsOpt            *WarningType `yaml:"dns_opt,omitempty"`
	DnsSearch         *WarningType `yaml:"dns_search,omitempty"`
	DomainName        *WarningType `yaml:"domainname,omitempty"`
	Extends           *WarningType `yaml:"extends,omitempty"`
	ExternalLinks     *WarningType `yaml:"external_links,omitempty"`
	ExtraHosts        *WarningType `yaml:"extra_hosts,omitempty"`
	GroupAdd          *WarningType `yaml:"group_add,omitempty"`
	Healthcheck       *WarningType `yaml:"healthcheck,omitempty"`
	Hostname          *WarningType `yaml:"hostname,omitempty"`
	Init              *WarningType `yaml:"init,omitempty"`
	Ipc               *WarningType `yaml:"ipc,omitempty"`
	Isolation         *WarningType `yaml:"isolation,omitempty"`
	Links             *WarningType `yaml:"links,omitempty"`
	Logging           *WarningType `yaml:"logging,omitempty"`
	Network_mode      *WarningType `yaml:"network_mode,omitempty"`
	Networks          *WarningType `yaml:"networks,omitempty"`
	MacAddress        *WarningType `yaml:"mac_address,omitempty"`
	MemSwappiness     *WarningType `yaml:"mem_swappiness,omitempty"`
	MemswapLimit      *WarningType `yaml:"memswap_limit,omitempty"`
	OomKillDisable    *WarningType `yaml:"oom_kill_disable,omitempty"`
	OomScoreAdj       *WarningType `yaml:"oom_score_adj,omitempty"`
	Pid               *WarningType `yaml:"pid,omitempty"`
	PidLimit          *WarningType `yaml:"pid_limit,omitempty"`
	Platform          *WarningType `yaml:"platform,omitempty"`
	Privileged        *WarningType `yaml:"privileged,omitempty"`
	Profiles          *WarningType `yaml:"profiles,omitempty"`
	PullPolicy        *WarningType `yaml:"pull_policy,omitempty"`
	ReadOnly          *WarningType `yaml:"read_only,omitempty"`
	Restart           *string      `yaml:"restart,omitempty"`
	Runtime           *WarningType `yaml:"runtime,omitempty"`
	Secrets           *WarningType `yaml:"secrets,omitempty"`
	SecurityOpt       *WarningType `yaml:"security_opt,omitempty"`
	ShmSize           *WarningType `yaml:"shm_size,omitempty"`
	StdinOpen         *WarningType `yaml:"stdin_open,omitempty"`
	StopSignal        *WarningType `yaml:"stop_signal,omitempty"`
	StorageOpts       *WarningType `yaml:"storage_opts,omitempty"`
	Sysctls           *WarningType `yaml:"sysctls,omitempty"`
	Tmpfs             *WarningType `yaml:"tmpfs,omitempty"`
	Tty               *WarningType `yaml:"tty,omitempty"`
	Ulimits           *WarningType `yaml:"ulimits,omitempty"`
	User              *WarningType `yaml:"user,omitempty"`
	UsernsMode        *WarningType `yaml:"userns_mode,omitempty"`
	VolumesFrom       *WarningType `yaml:"volumes_from,omitempty"`
}

type DeployInfoRaw struct {
	Replicas  int32        `yaml:"replicas,omitempty"`
	Resources ResourcesRaw `yaml:"resources,omitempty"`
	Labels    Labels       `yaml:"labels,omitempty"`

	EndpointMode   *WarningType `yaml:"endpoint_mode,omitempty"`
	Mode           *WarningType `yaml:"mode,omitempty"`
	Placement      *WarningType `yaml:"placement,omitempty"`
	Constraints    *WarningType `yaml:"constraints,omitempty"`
	Preferences    *WarningType `yaml:"preferences,omitempty"`
	RestartPolicy  *WarningType `yaml:"restart_policy,omitempty"`
	RollbackConfig *WarningType `yaml:"rollback_config,omitempty"`
	UpdateConfig   *WarningType `yaml:"update_config,omitempty"`
}

type PortRaw struct {
	ContainerPort int32
	HostPort      int32
	Protocol      apiv1.Protocol
}

type WarningType struct {
	used bool
}

type ResourcesRaw struct {
	Limits       DeployComposeResources `json:"limits,omitempty" yaml:"limits,omitempty"`
	Reservations DeployComposeResources `json:"reservations,omitempty" yaml:"reservations,omitempty"`
}

type DeployComposeResources struct {
	Cpus    Quantity     `json:"cpus,omitempty" yaml:"cpus,omitempty"`
	Memory  Quantity     `json:"memory,omitempty" yaml:"memory,omitempty"`
	Devices *WarningType `json:"devices,omitempty" yaml:"devices,omitempty"`
}

type VolumeTopLevel struct {
	Labels      Labels            `json:"labels,omitempty" yaml:"labels,omitempty"`
	Annotations Annotations       `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Name        string            `json:"name,omitempty" yaml:"name,omitempty"`
	Size        Quantity          `json:"size,omitempty" yaml:"size,omitempty"`
	Class       string            `json:"class,omitempty" yaml:"class,omitempty"`
	DriverOpts  map[string]string `json:"driver_opts,omitempty" yaml:"driver_opts,omitempty"`

	Driver   *WarningType `json:"driver,omitempty" yaml:"driver,omitempty"`
	External *WarningType `json:"external,omitempty" yaml:"external,omitempty"`
}
type RawMessage struct {
	unmarshal func(interface{}) error
}

func (s *Stack) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var stackRaw StackRaw
	err := unmarshal(&stackRaw)
	if err != nil {
		return err
	}

	s.Name = stackRaw.Name

	s.Namespace = stackRaw.Namespace

	s.Endpoints = stackRaw.Endpoints
	if endpoint, ok := s.Endpoints[""]; ok {
		s.Endpoints[s.Name] = endpoint
		delete(s.Endpoints, "")
	}

	if len(s.Endpoints) == 0 {
		s.Endpoints = getEndpointsFromPorts(stackRaw.Services)
	}
	volumes := make(map[string]*VolumeSpec)
	for volumeName, v := range stackRaw.Volumes {
		result := VolumeSpec{}
		if v == nil {
			result.Labels = make(Labels)
			result.Annotations = make(Annotations)
		} else {
			result.Labels = make(Labels)
			if v.Annotations == nil {
				result.Annotations = make(Annotations)
			} else {
				result.Annotations = v.Annotations
			}

			for key, value := range v.Labels {
				result.Annotations[key] = value
			}

			if v.Size.Value.Cmp(resource.MustParse("0")) > 0 {
				result.Size = v.Size
			}
			if v.DriverOpts != nil {
				for key, value := range v.DriverOpts {
					if key == "size" {
						qK8s, err := resource.ParseQuantity(value)
						if err != nil {
							return err
						}
						result.Size.Value = qK8s
					}
				}
			}
		}

		volumes[volumeName] = &result
	}

	s.Volumes = make(map[string]*VolumeSpec)
	for volumeName, volume := range volumes {
		s.Volumes[sanitizeName(volumeName)] = volume
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

func getEndpointsFromPorts(services map[string]*ServiceRaw) EndpointSpec {
	endpoints := make(EndpointSpec)
	for svcName, svc := range services {
		accessiblePorts := getAccessiblePorts(svc.Ports)
		if len(accessiblePorts) >= 2 {
			for _, p := range accessiblePorts {
				endpointName := fmt.Sprintf("%s-%d", svcName, p.HostPort)
				endpoints[endpointName] = Endpoint{
					Rules: []EndpointRule{
						{
							Path:    "/",
							Service: svcName,
							Port:    p.ContainerPort,
						},
					},
				}
			}
		}
	}
	return endpoints
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

	svc.Resources, err = unmarshalDeployResources(serviceRaw.Deploy, serviceRaw.Resources, serviceRaw.CpuCount, serviceRaw.Cpus, serviceRaw.MemLimit, serviceRaw.MemReservation)
	if err != nil {
		return nil, err
	}
	svc.Replicas, err = unmarshalDeployReplicas(serviceRaw.Deploy, serviceRaw.Scale, serviceRaw.Replicas)
	if err != nil {
		return nil, err
	}
	svc.Image = serviceRaw.Image
	svc.Build = serviceRaw.Build

	svc.CapAdd = serviceRaw.CapAdd
	if len(serviceRaw.CapAddSneakCase) > 0 {
		svc.CapAdd = serviceRaw.CapAddSneakCase
	}
	svc.CapDrop = serviceRaw.CapDrop
	if len(serviceRaw.CapDropSneakCase) > 0 {
		svc.CapDrop = serviceRaw.CapDropSneakCase
	}

	svc.Annotations = serviceRaw.Annotations
	if svc.Annotations == nil {
		svc.Annotations = make(Annotations)
	}
	for key, value := range unmarshalLabels(serviceRaw.Labels, serviceRaw.Deploy) {
		svc.Annotations[key] = value
	}

	if stack.IsCompose {
		if len(serviceRaw.Args.Values) > 0 {
			return nil, fmt.Errorf("Unsupported field for services.%s: 'args'", svcName)
		}
		svc.Entrypoint.Values = serviceRaw.Entrypoint.Values
		svc.Command.Values = serviceRaw.Command.Values

	} else { // isOktetoStack
		if len(serviceRaw.Entrypoint.Values) > 0 {
			return nil, fmt.Errorf("Unsupported field for services.%s: 'entrypoint'", svcName)
		}
		svc.Entrypoint.Values = serviceRaw.Command.Values
		if len(serviceRaw.Command.Values) == 1 {
			if strings.Contains(serviceRaw.Command.Values[0], " ") {
				svc.Entrypoint.Values = []string{"sh", "-c", serviceRaw.Command.Values[0]}
			}
		}
		svc.Command.Values = serviceRaw.Args.Values
	}

	svc.EnvFiles = serviceRaw.EnvFiles
	if len(serviceRaw.EnvFilesSneakCase) > 0 {
		svc.EnvFiles = serviceRaw.EnvFilesSneakCase
	}

	svc.Environment = serviceRaw.Environment

	svc.Public = serviceRaw.Public
	if !svc.Public && len(getAccessiblePorts(serviceRaw.Ports)) == 1 {
		svc.Public = true
	}
	for _, p := range serviceRaw.Ports {
		svc.Ports = append(svc.Ports, Port{Port: p.ContainerPort, Protocol: p.Protocol})
	}

	svc.Expose, err = unmarshalExpose(serviceRaw.Expose)
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
	for _, volume := range svc.VolumeMounts {
		if volume.LocalPath != "" && !(!FileExists(volume.LocalPath) || !strings.HasPrefix(volume.LocalPath, "/")) {
			return nil, fmt.Errorf("Named volume '%s' is used in service '%s' but no declaration was found in the volumes section.", volume.ToString(), svcName)
		}
	}

	svc.Workdir = serviceRaw.Workdir
	if serviceRaw.WorkingDirSneakCase != "" {
		svc.Workdir = serviceRaw.WorkingDirSneakCase
	}
	return svc, nil
}

func splitVolumesByType(volumes []StackVolume, s *Stack) ([]StackVolume, []StackVolume) {
	topLevelVolumes := make([]StackVolume, 0)
	mountedVolumes := make([]StackVolume, 0)
	for _, volume := range volumes {
		if volume.LocalPath == "" || isInVolumesTopLevelSection(volume.LocalPath, s) {
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
func (p *PortRaw) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rawPort string
	err := unmarshal(&rawPort)
	if err != nil {
		return fmt.Errorf("Port field is only supported in short syntax")
	}

	parts := strings.Split(rawPort, ":")
	var portString string
	var hostPortString string
	if len(parts) == 1 {
		if strings.Contains(portString, "-") {
			return fmt.Errorf("Can not convert %s. Range ports are not supported.", rawPort)
		}

		portString = parts[0]
	} else if len(parts) <= 3 {
		if strings.Contains(portString, "-") {
			return fmt.Errorf("Can not convert %s. Range ports are not supported.", rawPort)
		}

		portString = parts[len(parts)-1]

		hostPortString = strings.Join(parts[:len(parts)-1], ":")
		parts := strings.Split(hostPortString, ":")
		hostString := parts[len(parts)-1]
		port, err := strconv.Atoi(hostString)
		if err != nil {
			return fmt.Errorf("Can not convert %s to a port.", hostString)
		}
		p.HostPort = int32(port)
		if IsSkippablePort(p.HostPort) {
			p.HostPort = 0
		}
	} else {
		return fmt.Errorf(malformedPortForward, rawPort)
	}

	p.Protocol = apiv1.ProtocolTCP
	if strings.Contains(portString, "/") {
		portAndProtocol := strings.Split(portString, "/")
		portString = portAndProtocol[0]
		if protocol, err := getProtocol(portAndProtocol[1]); err == nil {
			p.Protocol = protocol
		} else {
			return fmt.Errorf("Can not convert %s. Only TCP ports are allowed.", portString)
		}
	}

	port, err := strconv.Atoi(portString)
	if err != nil {
		return fmt.Errorf("Can not convert %s to a port.", portString)
	}
	p.ContainerPort = int32(port)

	return nil
}

func IsSkippablePort(port int32) bool {
	skippablePorts := map[int32]string{3306: "MySQL", 1521: "OracleDB", 1830: "OracleDB", 5432: "PostgreSQL",
		1433: "SQL Server", 1434: "SQL Server", 7210: "MaxDB", 7473: "Neo4j", 7474: "Neo4j", 8529: "ArangoDB",
		7000: "Cassandra", 7001: "Cassandra", 9042: "Cassandra", 8086: "InfluxDB", 9200: "Elasticsearch", 9300: "Elasticsearch",
		5984: "CouchDB", 27017: "MongoDB", 27018: "MongoDB", 27019: "MongoDB", 28017: "MongoDB", 6379: "Redis",
		8087: "Riak", 8098: "Riak", 828015: "Rethink", 29015: "Rethink", 7574: "Solr", 8983: "Solr",
		2345: "Golang debugger", 5858: "Node debugger", 9229: "Node debugger", 5005: "Java debugger", 1234: "Ruby debugger",
		4444: "Python pdb", 5678: "Python debugpy"}
	if _, ok := skippablePorts[port]; ok {
		return true
	}
	return false
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (p *Port) MarshalYAML() (interface{}, error) {
	return Port{Port: p.Port, Protocol: p.Protocol}, nil
}

func unmarshalDeployResources(deployInfo *DeployInfoRaw, resources *StackResources, cpuCount, cpus, memLimit, memReservation Quantity) (*StackResources, error) {
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

	return resources, nil
}

func unmarshalDeployReplicas(deployInfo *DeployInfoRaw, scale, replicas int32) (int32, error) {
	var finalReplicas int32
	finalReplicas = 1
	if deployInfo != nil {
		if deployInfo.Replicas > replicas {
			finalReplicas = deployInfo.Replicas
		}
	}
	if scale > finalReplicas {
		finalReplicas = scale
	}
	if replicas > finalReplicas {
		finalReplicas = replicas
	}

	return finalReplicas, nil
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

func unmarshalExpose(raw *RawMessage) ([]int32, error) {
	exposeInInt := make([]int32, 0)
	if raw == nil {
		return exposeInInt, nil
	}
	err := raw.unmarshal(&exposeInInt)
	if err == nil {
		return exposeInInt, nil
	}
	var exposeInString []string
	err = raw.unmarshal(&exposeInString)
	if err != nil {
		return exposeInInt, err
	}

	for _, expose := range exposeInString {
		if strings.Contains(expose, "-") {
			return exposeInInt, fmt.Errorf("Can not convert %s. Range ports are not supported.", expose)
		}
		parts := strings.Split(expose, ":")
		var portString string
		if len(parts) == 1 {
			portString = parts[0]
		} else if len(parts) <= 3 {
			portString = parts[len(parts)-1]
		} else {
			return exposeInInt, fmt.Errorf(malformedPortForward, expose)
		}
		port, err := strconv.Atoi(portString)
		if err != nil {
			return exposeInInt, err
		}
		exposeInInt = append(exposeInInt, int32(port))
	}
	return exposeInInt, nil
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

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (v *StackVolume) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw string
	err := unmarshal(&raw)
	if err != nil {
		return err
	}

	parts := strings.SplitN(raw, ":", 2)
	if len(parts) == 2 {
		v.LocalPath, err = ExpandEnv(parts[0])
		v.LocalPath = sanitizeName(v.LocalPath)
		if err != nil {
			return err
		}
		v.RemotePath = parts[1]
	} else {
		v.RemotePath = parts[0]
	}
	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (v StackVolume) MarshalYAML() (interface{}, error) {
	return v.RemotePath, nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (v StackVolume) ToString() string {
	if v.LocalPath != "" {
		return fmt.Sprintf("%s:%s", v.LocalPath, v.RemotePath)
	}
	return v.RemotePath
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

func shouldBeSanitized(name string) bool {
	return strings.Contains(name, " ") || strings.Contains(name, "_")
}

func sanitizeName(name string) string {
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")
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
	if svcInfo.DependsOn != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].depends_on", svcName))
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
	if svcInfo.Healthcheck != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].healthcheck", svcName))
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
	if svcInfo.Restart != nil {
		if *svcInfo.Restart != "always" {
			notSupported = append(notSupported, fmt.Sprintf("services[%s].restart", svcName))
		}
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
	if svcInfo.User != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].user", svcName))
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
	if deploy.RestartPolicy != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].deploy.restart_policy", svcName))
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
			if key != "size" {
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
