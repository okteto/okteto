// Copyright 2022 The Okteto Authors
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

const malformedGlobalForward = "Wrong global forward syntax '%s', must be of the form 'localPort:serviceName:remotePort'"

// Forward represents a port forwarding definition
type GlobalForward struct {
	Local       int               `json:"localPort" yaml:"localPort"`
	Remote      int               `json:"remotePort" yaml:"remotePort"`
	ServiceName string            `json:"name" yaml:"name"`
	Labels      map[string]string `json:"labels" yaml:"labels"`
	IsAdded     bool              `json:"-" yaml:"-"`
}

type GlobalForwardRaw struct {
	Local       int               `json:"localPort" yaml:"localPort"`
	Remote      int               `json:"remotePort" yaml:"remotePort"`
	ServiceName string            `json:"name" yaml:"name"`
	Labels      map[string]string `json:"labels" yaml:"labels"`
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg for port forwards.
// It supports the following options:
// - int:serviceName:int
// Anything else will result in an error
func (gf *GlobalForward) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw string
	err := unmarshal(&raw)
	if err != nil {
		return gf.UnmarshalExtendedForm(unmarshal)
	}

	parts := strings.Split(raw, ":")
	if len(parts) != 3 {
		return fmt.Errorf(malformedGlobalForward, raw)
	}

	gf.ServiceName = parts[1]

	localPort, err := strconv.Atoi(parts[0])
	if err != nil {
		return fmt.Errorf("Cannot convert local port '%s' in port-forward '%s'", parts[0], raw)
	}
	gf.Local = localPort

	remotePort, err := strconv.Atoi(parts[2])
	if err != nil {
		return fmt.Errorf("Cannot convert local port '%s' in port-forward '%s'", parts[0], raw)
	}
	gf.Remote = remotePort

	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (gf GlobalForward) MarshalYAML() (interface{}, error) {
	return gf.String(), nil
}

func (gf GlobalForward) String() string {
	return fmt.Sprintf("%d:%s:%d", gf.Local, gf.ServiceName, gf.Remote)
}

func (gf *GlobalForward) less(c *GlobalForward) bool {
	return gf.Local < c.Local
}

func (gf *GlobalForward) UnmarshalExtendedForm(unmarshal func(interface{}) error) error {
	var rawGlobalForward GlobalForwardRaw
	err := unmarshal(&rawGlobalForward)
	if err != nil {
		return err
	}
	gf.Local = rawGlobalForward.Local
	gf.Remote = rawGlobalForward.Remote
	gf.ServiceName = rawGlobalForward.ServiceName
	gf.Labels = rawGlobalForward.Labels

	if gf.Labels != nil && gf.ServiceName != "" {
		return fmt.Errorf("Can not use ServiceName and Labels to specify the service.\nUse either the service name or labels to get the service to expose.")
	}
	return nil
}
