package model

import (
	"encoding/base64"
	"fmt"

	yaml "gopkg.in/yaml.v2"
)

//Compose represents a docker-compose.yml file
type Compose struct {
	Version  string
	Services map[string]*ComposeService
}

//ComposeService represents a service in a docker-compose.yml file
type ComposeService struct {
	Image       string
	Command     string         `yaml:"command,omitempty"`
	Ports       []*string      `yaml:"ports,omitempty"`
	Environment []*string      `yaml:"environment,omitempty"`
	Logging     *LoggingDriver `yaml:"logging,omitempty"`
	Restart     string         `yaml:"restart,omitempty"`
	DNSSearch   []*string      `yaml:"dns_search,omitempty"`
}

//LoggingDriver represents a docker logging driver
type LoggingDriver struct {
	Driver  string            `yaml:"driver,omitempty"`
	Options map[string]string `yaml:"options,omitempty"`
}

//CreateCompose returns a compose base64 encoded given a service object
func CreateCompose(s *Service, e *Environment, prefix string) (string, error) {
	domain := e.Domain()
	compose := &Compose{
		Version:  "3.4",
		Services: make(map[string]*ComposeService),
	}
	for cName, c := range s.Containers {
		compose.Services[cName] = &ComposeService{
			Image:       c.Image,
			Command:     c.Command,
			Ports:       []*string{},
			Environment: []*string{},
			Restart:     "on-failure",
			DNSSearch:   []*string{&domain},
		}

		compose.Services[cName].Ports = parsePorts(s.Stateful, c.Ports)

		for _, e := range c.Environment {
			composeEnvVar := fmt.Sprintf("%s=%s", e.Name, e.Value)
			compose.Services[cName].Environment = append(
				compose.Services[cName].Environment,
				&composeEnvVar,
			)
		}
		compose.Services[cName].Logging = &LoggingDriver{
			Driver: "awslogs",
			Options: map[string]string{
				"awslogs-region": e.Provider.Region,
				"awslogs-group":  fmt.Sprintf("%s-%s-%s", prefix, s.Name, s.ID),
				"tag":            fmt.Sprintf("%s-${INSTANCE_ID}", cName),
			},
		}
	}
	composeBytes, err := yaml.Marshal(compose)
	if err != nil {
		return "", err
	}
	composeEncoded := base64.StdEncoding.EncodeToString(composeBytes)
	return composeEncoded, nil
}

func parsePorts(stateful bool, ports []*Port) []*string {
	parsedPorts := make([]*string, 0)

	for _, p := range ports {
		var composePort string
		if stateful {
			composePort = fmt.Sprintf("%s:%s", p.Port, p.InstancePort)
		} else {
			composePort = fmt.Sprintf("%s:%s", p.InstancePort, p.InstancePort)
		}
		parsedPorts = append(parsedPorts, &composePort)
	}

	removeDuplicates(&parsedPorts)
	return parsedPorts
}

func removeDuplicates(xs *[]*string) {
	found := make(map[string]bool)
	j := 0
	for i, x := range *xs {
		if !found[*x] {
			found[*x] = true
			(*xs)[j] = (*xs)[i]
			j++
		}
	}
	*xs = (*xs)[:j]
}
