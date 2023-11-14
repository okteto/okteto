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

const malformedGlobalForward = "Wrong global forward syntax '%s', must be of the form 'localPort:serviceName:remotePort'"

// GlobalForward forwards represents a port forwarding definition
type GlobalForward struct {
	Labels      map[string]string `json:"labels" yaml:"labels"`
	ServiceName string            `json:"name" yaml:"name"`
	Local       int               `json:"localPort" yaml:"localPort"`
	Remote      int               `json:"remotePort" yaml:"remotePort"`
	IsAdded     bool              `json:"-" yaml:"-"`
}

type GlobalForwardRaw struct {
	Labels      map[string]string `json:"labels" yaml:"labels"`
	ServiceName string            `json:"name" yaml:"name"`
	Local       int               `json:"localPort" yaml:"localPort"`
	Remote      int               `json:"remotePort" yaml:"remotePort"`
}

func (gf GlobalForward) String() string {
	return fmt.Sprintf("%d:%s:%d", gf.Local, gf.ServiceName, gf.Remote)
}

func (gf *GlobalForward) less(c *GlobalForward) bool {
	return gf.Local < c.Local
}
