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

	validGlobalForwardParts := 3
	parts := strings.Split(raw, ":")
	if len(parts) != validGlobalForwardParts {
		return fmt.Errorf(malformedGlobalForward, raw)
	}

	svcName := parts[1]
	if svcName == "" {
		return fmt.Errorf("service name cannot be empty")
	}

	gf.ServiceName = svcName

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
		return fmt.Errorf("Can not use both ServiceName and Labels to select the service.\nUse either the service name or labels to select the service to be exposed.")
	}

	if gf.Labels == nil && gf.ServiceName == "" {
		return fmt.Errorf("You need to specify either ServiceName or labels to select the service.")
	}

	return nil
}
