//Stack represents an okteto stack
type StackRaw struct {
	Name       string                 `yaml:"name"`
	Namespace  string                 `yaml:"namespace,omitempty"`
	Services   map[string]*ServiceRaw `yaml:"services,omitempty"`

}

//Service represents an okteto stack service
type ServiceRaw struct {
	Deploy          *DeployInfoRaw     `yaml:"deploy,omitempty"`                                       // Lo de capabilities
	Build           *BuildInfo         `yaml:"build,omitempty"`                                        // Done
	CapAdd          []apiv1.Capability `yaml:"cap_add,omitempty"`                                      // Done
	CapDrop         []apiv1.Capability `yaml:"cap_drop,omitempty"`                                     // Done
	Command         Args               `yaml:"command,omitempty"`                                      // Done
	Entrypoint      Command            `yaml:"entrypoint,omitempty"`                                   // Done
	EnvFiles        []string           `yaml:"env_file,omitempty"`                                     // Done
	Environment     *RawMessage        `yaml:"environment,omitempty"`                                  // Done with Envs
	Expose          []int32            `yaml:"expose,omitempty"`                                       // Done
	Image           string             `yaml:"image,omitempty"`                                        // Done
	Labels          *RawMessage        `json:"labels,omitempty" yaml:"labels,omitempty"`               // Accept also list
	Annotations     map[string]string  `json:"annotations,omitempty" yaml:"annotations,omitempty"`     // Done
	Xannotations    map[string]string  `json:"x-annotations,omitempty" yaml:"x-annotations,omitempty"` // Done
	Ports           []Port             `yaml:"ports,omitempty"`                                        // Done
	Scale           int32              `yaml:"scale"`                                                  // Done
	StopGracePeriod *RawMessage        `yaml:"stop_grace_period,omitempty"`                            // Change to duration
	Volumes         []VolumeStack      `yaml:"volumes,omitempty"`                                      // Redo
	WorkingDir      string             `yaml:"working_dir,omitempty"`

	Public    bool             `yaml:"public,omitempty"`
	Replicas  int32            `yaml:"replicas"`
	Resources ServiceResources `yaml:"resources,omitempty"`
}

type DeployInfoRaw struct {
	Replicas  int32        `yaml:"replicas,omitempty"`
	Resources ResourcesRaw `yaml:"resources,omitempty"`

	EndpointMode   *WarningType `yaml:"endpoint_mode,omitempty"`
	Labels         *WarningType `yaml:"labels,omitempty"`
	Mode           *WarningType `yaml:"mode,omitempty"`
	Placement      *WarningType `yaml:"placement,omitempty"`
	Constraints    *WarningType `yaml:"constraints,omitempty"`
	Preferences    *WarningType `yaml:"preferences,omitempty"`
	RestartPolicy  *WarningType `yaml:"restart_policy,omitempty"`
	RollbackConfig *WarningType `yaml:"rollback_config,omitempty"`
	UpdateConfig   *WarningType `yaml:"update_config,omitempty"`
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
	s.Services = make(map[string]*Service)
	for svcName, svcRaw := range stackRaw.Services {
		s.Services[svcName], err = svcRaw.ToService(svcName)
		if err != nil {
			return err
		}
	}
	stackRaw.Warnings = make([]string, 0)

	displayNotSupportedFields(&stackRaw)
	return nil
}

func (serviceRaw *ServiceRaw) ToService(svcName string) (*Service, error) {
	s := &Service{}
	var err error
	s.Deploy, err = unmarshalDeploy(serviceRaw.Deploy, serviceRaw.Scale, serviceRaw.Replicas, serviceRaw.Resources)
	if err != nil {
		return nil, err
	}
	s.Image = serviceRaw.Image
	s.Build = serviceRaw.Build

	s.CapAdd = serviceRaw.CapAdd
	s.CapDrop = serviceRaw.CapDrop
	s.EnvFiles = serviceRaw.EnvFiles

	s.Environment, err = unmarshalEnvs(serviceRaw.Environment)
	if err != nil {
		return nil, err
	}

	s.Ports = serviceRaw.Ports
	s.Annotations = serviceRaw.Annotations
	s.Public = serviceRaw.Public
	return s, nil
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
func (p *Port) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rawPort string
	err := unmarshal(&rawPort)
	if err != nil {
		return fmt.Errorf("Port field is only supported in short syntax")
	}

	parts := strings.Split(rawPort, ":")
	var portString string
	if len(parts) == 1 {
		portString = parts[0]
		if strings.Contains(portString, "-") {
			return fmt.Errorf("Can not convert %s. Range ports are not supported.", rawPort) // TODO: change message
		}
	} else if len(parts) <= 3 {
		portString = parts[len(parts)-1]
		if strings.Contains(portString, "-") {
			return fmt.Errorf("Can not convert %s. Range ports are not supported.", rawPort) // TODO: change message
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
	p.Port = int32(port)
	p.Public = isPublicPort(p.Port)

	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (p *Port) MarshalYAML() (interface{}, error) {
	return Port{Port: p.Port, Public: p.Public}, nil
}

func unmarshalDeploy(deployInfo *DeployInfoRaw, scale int32, replicas int32, resources ServiceResources) (*DeployInfo, error) {
	deploy := &DeployInfo{Replicas: 1, Resources: ResourceRequirements{Limits: make(map[apiv1.ResourceName]resource.Quantity, 0),
		Requests: make(map[apiv1.ResourceName]resource.Quantity, 0)}}
	if deployInfo != nil {
		if deployInfo.Replicas > deploy.Replicas {
			deploy.Replicas = deployInfo.Replicas
		}
		deploy.Resources.Limits = deployInfo.Resources.Limits.toResourceList()
		deploy.Resources.Requests = deployInfo.Resources.Reservations.toResourceList()
	}

	if scale > deploy.Replicas {
		deploy.Replicas = scale
	}
	if replicas > deploy.Replicas {
		deploy.Replicas = replicas
	}

	if !resources.CPU.Value.IsZero() {
		deploy.Resources.Requests[apiv1.ResourceCPU] = resources.CPU.Value
	}
	if !resources.Memory.Value.IsZero() {
		deploy.Resources.Requests[apiv1.ResourceMemory] = resources.Memory.Value
	}
	if !resources.Storage.Size.Value.IsZero() {
		deploy.Resources.Requests[apiv1.ResourceStorage] = resources.Storage.Size.Value
	}
	return deploy, nil
}

func (r DeployComposeResources) toResourceList() ResourceList {
	resources := make(map[apiv1.ResourceName]resource.Quantity, 0)
	if !r.Cpus.Value.IsZero() {
		resources[apiv1.ResourceCPU] = r.Cpus.Value
	}
	if !r.Memory.Value.IsZero() {
		resources[apiv1.ResourceMemory] = r.Memory.Value
	}
	return resources
}

func isPublicPort(port int32) bool {
	return (80 <= port && port <= 90) || (8000 <= port && port <= 9000) || port == 3000 || port == 5000
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
func getTopLevelNotSupportedFields(s *StackRaw) []string {
	notSupported := make([]string, 0)
	if s.Networks != nil {
		notSupported = append(notSupported, "networks")
	}
func getServiceNotSupportedFields(svcName string, svcInfo *ServiceRaw) []string {
	notSupported := make([]string, 0)
	if svcInfo.Deploy != nil {
		notSupported = append(notSupported, getDeployNotSupportedFields(svcName, svcInfo.Deploy)...)
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
	if deploy.Labels != nil {
		notSupported = append(notSupported, fmt.Sprintf("services[%s].deploy.labels", svcName))
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
