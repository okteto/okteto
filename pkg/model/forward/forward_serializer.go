// Copyright 2023 The Okteto Authors
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

package forward

import (
	"fmt"
	"strconv"
	"strings"
)

type Raw struct {
	Labels      map[string]string `json:"labels" yaml:"labels"`
	ServiceName string            `json:"name" yaml:"name"`
	Local       int               `json:"localPort" yaml:"localPort"`
	Remote      int               `json:"remotePort" yaml:"remotePort"`
	Service     bool              `json:"-" yaml:"-"`
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

	maxForwardParts := 3
	minForwardParts := 2
	parts := strings.Split(raw, ":")
	if len(parts) < minForwardParts || len(parts) > maxForwardParts {
		return fmt.Errorf(MalformedPortForward, raw)
	}

	localPort, err := strconv.Atoi(parts[0])
	if err != nil {
		return fmt.Errorf("Cannot convert local port '%s' in port-forward '%s'", parts[0], raw)
	}
	f.Local = localPort

	if len(parts) == minForwardParts {
		p, err := strconv.Atoi(parts[1])
		if err != nil {
			return fmt.Errorf(MalformedPortForward, raw)
		}

		f.Remote = p
		return nil
	}

	f.Service = true
	f.ServiceName = parts[1]
	p, err := strconv.Atoi(parts[2])
	if err != nil {
		return fmt.Errorf(MalformedPortForward, raw)
	}

	f.Remote = p
	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (f Forward) MarshalYAML() (interface{}, error) {
	return f.String(), nil
}

func (f *Forward) UnmarshalExtendedForm(unmarshal func(interface{}) error) error {
	var rawForward Raw
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
	if f.Labels != nil && f.ServiceName != "" {
		return fmt.Errorf("Can not use ServiceName and Labels to specify the service.\nUse either the service name or labels to get the service to expose.")
	}
	return nil
}
