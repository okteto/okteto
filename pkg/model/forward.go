package model

import (
	"fmt"
	"strconv"
	"strings"
)

// Forward represents a port forwarding definition
type Forward struct {
	Local       int
	Remote      int
	Service     bool   `json:"-" yaml:"-"`
	ServiceName string `json:"-" yaml:"-"`
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg for port forwards.
// It supports the following options:
// - int:int
// - int:serviceName
// - int:serviceName:int
// Anything else will result in an error
func (f *Forward) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw string
	err := unmarshal(&raw)
	if err != nil {
		return err
	}

	parts := strings.Split(raw, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return fmt.Errorf("Wrong port-forward syntax '%s', must be of the form 'localPort:remotePort', 'localPort:serviceName', or 'localPort:serviceName:remotePort' ", raw)
	}

	localPort, err := strconv.Atoi(parts[0])
	if err != nil {
		return fmt.Errorf("Cannot convert local port '%s' in port-forward '%s'", parts[0], raw)
	}
	f.Local = localPort

	remotePort, err := strconv.Atoi(parts[1])
	if err == nil {
		if len(parts) == 3 {
			return fmt.Errorf("Wrong port-forward syntax '%s', must be of the form 'localPort:remotePort', 'localPort:serviceName', or 'localPort:serviceName:remotePort' ", raw)
		}

		f.Remote = remotePort
		f.Service = false
		return nil
	}

	f.Service = true
	f.ServiceName = parts[1]

	if len(parts) == 3 {
		p, err := strconv.Atoi(parts[2])
		if err != nil {
			return fmt.Errorf("Wrong port-forward syntax '%s', must be of the form 'localPort:remotePort', 'localPort:serviceName', or 'localPort:serviceName:remotePort' ", raw)
		}

		f.Remote = p
	}

	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (f Forward) MarshalYAML() (interface{}, error) {
	if f.Service {
		if f.Remote > 0 {
			return fmt.Sprintf("%d:%s:%d", f.Local, f.ServiceName, f.Remote), nil
		}

		return fmt.Sprintf("%d:%s", f.Local, f.ServiceName), nil
	}

	return fmt.Sprintf("%d:%d", f.Local, f.Remote), nil
}
