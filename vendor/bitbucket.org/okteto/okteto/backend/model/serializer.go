package model

//MarshalYAML serializes p provided into a YAML document. The return value is a string.
import (
	"bytes"
	"fmt"
	"strings"

	yaml "gopkg.in/yaml.v2"
)

const (
	//HTTP is the http protocol
	HTTP = "HTTP"
	//HTTPS is the https protocol
	HTTPS = "HTTPS"
	//TCP is the tcp protocol
	TCP = "TCP"
	//SSL is the ssl protocol
	SSL = "SSL"
)

//MarshalYAML fails if p has missing required values
func (p *Port) MarshalYAML() (interface{}, error) {
	if p.Protocol == "" || p.Port == "" || p.InstanceProtocol == "" || p.InstancePort == "" {
		return "", fmt.Errorf("missing values")
	}

	var buffer bytes.Buffer
	buffer.WriteString(strings.ToLower(p.Protocol))
	buffer.WriteString(":")
	buffer.WriteString(p.Port)
	buffer.WriteString(":")
	buffer.WriteString(strings.ToLower(p.InstanceProtocol))
	buffer.WriteString(":")
	buffer.WriteString(p.InstancePort)
	if p.Certificate != "" {
		buffer.WriteString(":")
		buffer.WriteString(p.Certificate)
	}

	return buffer.String(), nil
}

//UnmarshalYAML parses the yaml element and sets the values of p; it will return an error if the parsing fails, or
//if the format is incorrect
func (p *Port) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var port string
	if err := unmarshal(&port); err != nil {
		return err
	}

	parts := strings.SplitN(port, ":", 5)
	certificate := ""
	if len(parts) != 4 && len(parts) != 5 {
		return fmt.Errorf("invalid port syntax")
	}
	if len(parts) == 5 {
		certificate = parts[4]
	}

	protocol := strings.ToUpper(parts[0])
	instanceProtocol := strings.ToUpper(parts[2])
	if protocol != HTTP && protocol != HTTPS && protocol != TCP && protocol != SSL {
		return fmt.Errorf("unsupported port protocol mapping from '%s' to '%s'", parts[0], parts[2])
	}
	if protocol == HTTP && protocol != instanceProtocol {
		return fmt.Errorf("unsupported port protocol mapping from '%s' to '%s'", parts[0], parts[2])
	}
	if protocol == HTTPS && instanceProtocol != HTTPS && instanceProtocol != HTTP {
		return fmt.Errorf("unsupported port protocol mapping from '%s' to '%s'", parts[0], parts[2])
	}
	if protocol == TCP && protocol != instanceProtocol {
		return fmt.Errorf("unsupported port protocol mapping from '%s' to '%s'", parts[0], parts[2])
	}
	if protocol == SSL && instanceProtocol != SSL && instanceProtocol != TCP {
		return fmt.Errorf("unsupported port protocol mapping from '%s' to '%s'", parts[0], parts[2])
	}

	p.Protocol = protocol
	p.Port = parts[1]
	p.InstanceProtocol = instanceProtocol
	p.InstancePort = parts[3]
	p.Certificate = certificate
	return nil
}

//MarshalYAML serializes e into a YAML document. The return value is a string; It will fail if e has an empty name.
func (e *EnvVar) MarshalYAML() (interface{}, error) {
	if e.Name == "" {
		return "", fmt.Errorf("missing values")
	}

	var buffer bytes.Buffer
	buffer.WriteString(e.Name)
	buffer.WriteString("=")
	if e.Value != "" {
		buffer.WriteString(e.Value)
	}

	return buffer.String(), nil
}

//UnmarshalYAML parses the yaml element and sets the values of e; it will return an error if the parsing fails, or
//if the format is incorrect
func (e *EnvVar) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var envvar string
	if err := unmarshal(&envvar); err != nil {
		return err
	}

	envvar = strings.TrimPrefix(envvar, "=")

	parts := strings.Split(envvar, "=")
	if len(parts) != 2 {
		return fmt.Errorf("Invalid environment variable syntax")
	}

	e.Name = parts[0]
	e.Value = parts[1]
	return nil
}

func (s *Service) String() string {
	yamlBytes, err := yaml.Marshal(s)
	if err != nil {
		return "service-error"
	}

	return string(yamlBytes[:])
}
