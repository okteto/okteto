package model

import (
	"os"
	"strings"
)

type serialMount struct {
	Source string `json:"source,omitempty" yaml:"source,omitempty"`
	Path   string `json:"path" yaml:"path,omitempty"`
	Size   string `json:"size,omitempty" yaml:"size,omitempty"`
}

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
		if strings.HasPrefix(parts[1], "$") {
			e.Value = os.ExpandEnv(parts[1])
			return nil
		}

		e.Value = parts[1]
		return nil
	}

	val := os.ExpandEnv(parts[0])
	if val != parts[0] {
		e.Value = val
	}

	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (e *EnvVar) MarshalYAML() (interface{}, error) {
	return e.Name + "=" + e.Value, nil
}
