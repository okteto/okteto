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
)

const MalformedPortForward = "wrong port-forward syntax '%s', must be of the form 'localPort:remotePort' or 'localPort:serviceName:remotePort'"

// Forward represents a port forwarding definition
type Forward struct {
	Labels      map[string]string `json:"labels" yaml:"labels"`
	ServiceName string            `json:"name" yaml:"name"`
	Local       int               `json:"localPort" yaml:"localPort"`
	Remote      int               `json:"remotePort" yaml:"remotePort"`
	Service     bool              `json:"-" yaml:"-"`
	IsGlobal    bool              `json:"-" yaml:"-"`
}

func (f Forward) String() string {
	if f.Service {
		return fmt.Sprintf("%d:%s:%d", f.Local, f.ServiceName, f.Remote)
	}

	return fmt.Sprintf("%d:%d", f.Local, f.Remote)
}

func (f *Forward) Less(c *Forward) bool {
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
