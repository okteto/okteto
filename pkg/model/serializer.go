package model

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	apiv1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
)

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (e *EnvVar) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw string
	err := unmarshal(&raw)
	if err != nil {
		return err
	}

	parts := strings.SplitN(raw, "=", 2)
	e.Name = parts[0]
	if len(parts) == 2 {
		e.Value = os.ExpandEnv(parts[1])
		return nil
	}

	e.Name = os.ExpandEnv(parts[0])
	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (e EnvVar) MarshalYAML() (interface{}, error) {
	return e.Name + "=" + e.Value, nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (f *Forward) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw string
	err := unmarshal(&raw)
	if err != nil {
		return err
	}

	parts := strings.SplitN(raw, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("Wrong port-forward syntax '%s', must be of the form 'localPort:RemotePort'", raw)
	}
	localPort, err := strconv.Atoi(parts[0])
	if err != nil {
		return fmt.Errorf("Cannot convert remote port '%s' in port-forward '%s'", parts[0], raw)
	}
	remotePort, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("Cannot convert remote port '%s' in port-forward '%s'", parts[1], raw)
	}
	f.Local = localPort
	f.Remote = remotePort
	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (f Forward) MarshalYAML() (interface{}, error) {
	return fmt.Sprintf("%d:%d", f.Local, f.Remote), nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (r *ResourceList) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw map[apiv1.ResourceName]string
	err := unmarshal(&raw)
	if err != nil {
		return err
	}

	for k, v := range raw {
		parsed, err := resource.ParseQuantity(v)
		if err != nil {
			return err
		}

		(*r)[k] = parsed
	}

	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (r ResourceList) MarshalYAML() (interface{}, error) {
	m := make(map[apiv1.ResourceName]string)
	for k, v := range r {
		m[k] = v.String()
	}

	return m, nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (v *Volume) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw string
	err := unmarshal(&raw)
	if err != nil {
		return err
	}

	parts := strings.SplitN(raw, ":", 2)
	if len(parts) == 2 {
		v.SubPath = parts[0]
		v.MountPath = parts[1]
	} else {
		v.MountPath = parts[0]
	}
	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (v Volume) MarshalYAML() (interface{}, error) {
	if v.SubPath == "" {
		return v.MountPath, nil
	}
	return v.SubPath + ":" + v.MountPath, nil
}
