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
func (s *Secret) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw string
	err := unmarshal(&raw)
	if err != nil {
		return err
	}

	parts := strings.Split(raw, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return fmt.Errorf("secrets must follow the syntax 'LOCAL_PATH:REMOTE_PATH:MODE'")
	}
	s.LocalPath = parts[0]
	if err := checkFileAndNotDirectory(s.LocalPath); err != nil {
		return err
	}
	s.RemotePath = parts[1]
	if !strings.HasPrefix(s.RemotePath, "/") {
		return fmt.Errorf("Secret remote path '%s' must be an absolute path", s.RemotePath)
	}
	if len(parts) == 3 {
		mode, err := strconv.ParseInt(parts[2], 8, 32)
		if err != nil {
			return fmt.Errorf("error parsing secret '%s' mode: %s", parts[0], err)
		}
		s.Mode = int32(mode)
	} else {
		s.Mode = 420
	}
	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (s Secret) MarshalYAML() (interface{}, error) {
	if s.Mode == 420 {
		return fmt.Sprintf("%s:%s:%s", s.LocalPath, s.RemotePath, strconv.FormatInt(int64(s.Mode), 8)), nil
	}
	return fmt.Sprintf("%s:%s", s.LocalPath, s.RemotePath), nil
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
		return fmt.Errorf("Cannot convert local port '%s' in port-forward '%s'", parts[0], raw)
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
func (f *Reverse) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw string
	err := unmarshal(&raw)
	if err != nil {
		return err
	}

	parts := strings.SplitN(raw, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("Wrong port-forward syntax '%s', must be of the form 'localPort:RemotePort'", raw)
	}
	remotePort, err := strconv.Atoi(parts[0])
	if err != nil {
		return fmt.Errorf("Cannot convert remote port '%s' in reverse '%s'", parts[0], raw)
	}

	localPort, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("Cannot convert local port '%s' in reverse '%s'", parts[1], raw)
	}

	f.Local = localPort
	f.Remote = remotePort
	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (f Reverse) MarshalYAML() (interface{}, error) {
	return fmt.Sprintf("%d:%d", f.Remote, f.Local), nil
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

func checkFileAndNotDirectory(path string) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return err
	}
	if fileInfo.Mode().IsRegular() {
		return nil
	}
	return fmt.Errorf("Secret '%s' is not a regular file", path)
}
