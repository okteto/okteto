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
)

const malformedPortForward = "Wrong port-forward syntax '%s', must be of the form 'localPort:remotePort' or 'localPort:serviceName:remotePort'"

// Forward represents a port forwarding definition
type Forward struct {
	Local       int               `json:"localPort" yaml:"localPort"`
	Remote      int               `json:"remotePort" yaml:"remotePort"`
	Service     bool              `json:"-" yaml:"-"`
	ServiceName string            `json:"name" yaml:"name"`
	Labels      map[string]string `json:"labels" yaml:"labels"`
}

type ForwardRaw struct {
	Local       int               `json:"localPort" yaml:"localPort"`
	Remote      int               `json:"remotePort" yaml:"remotePort"`
	Service     bool              `json:"-" yaml:"-"`
	ServiceName string            `json:"name" yaml:"name"`
	Labels      map[string]string `json:"labels" yaml:"labels"`
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg for port forwards.
// It supports the following options:
// - int:int
// - int:serviceName:int
// Anything else will result in an error
func (f *Forward) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw string
	err := unmarshal(&raw)
	if err != nil {
		return f.UnmarshalExtendedForm(unmarshal)
	}

	parts := strings.Split(raw, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return fmt.Errorf(malformedPortForward, raw)
	}

	localPort, err := strconv.Atoi(parts[0])
	if err != nil {
		return fmt.Errorf("Cannot convert local port '%s' in port-forward '%s'", parts[0], raw)
	}
	f.Local = localPort

	if len(parts) == 2 {
		p, err := strconv.Atoi(parts[1])
		if err != nil {
			return fmt.Errorf(malformedPortForward, raw)
		}

		f.Remote = p
		return nil
	}

	f.Service = true
	f.ServiceName = parts[1]
	p, err := strconv.Atoi(parts[2])
	if err != nil {
		return fmt.Errorf(malformedPortForward, raw)
	}

	f.Remote = p
	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (f Forward) MarshalYAML() (interface{}, error) {
	return f.String(), nil
}

func (f Forward) String() string {
	if f.Service {
		return fmt.Sprintf("%d:%s:%d", f.Local, f.ServiceName, f.Remote)
	}

	return fmt.Sprintf("%d:%d", f.Local, f.Remote)
}

func (f *Forward) less(c *Forward) bool {
	if !f.Service && !c.Service {
		return f.Local < c.Local
	}

	// a non-service always goes first
	if !f.Service && c.Service {
		return true
	}

	if f.Service && !c.Service {
		return false
	}

	return f.Local < c.Local
}

func (f *Forward) UnmarshalExtendedForm(unmarshal func(interface{}) error) error {
	var rawForward ForwardRaw
	err := unmarshal(&rawForward)
	if err != nil {
		return err
	}
	f.Local = rawForward.Local
	f.Remote = rawForward.Remote
	f.ServiceName = rawForward.ServiceName
	f.Labels = rawForward.Labels
	if len(rawForward.Labels) != 0 || rawForward.ServiceName != "" {
		f.Service = true
	}

	return nil
}
