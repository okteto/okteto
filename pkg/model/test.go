// Copyright 2024 The Okteto Authors
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
	"strings"

	"github.com/okteto/okteto/pkg/vars"
)

type Test struct {
	Image     string        `yaml:"image,omitempty"`
	Context   string        `yaml:"context,omitempty"`
	Commands  []TestCommand `yaml:"commands,omitempty"`
	DependsOn []string      `yaml:"depends_on,omitempty"`
	Caches    []string      `yaml:"caches,omitempty"`
	Artifacts []Artifact    `yaml:"artifacts,omitempty"`
	Hosts     []Host        `yaml:"hosts,omitempty"`
}

type Host struct {
	Hostname string `yaml:"hostname,omitempty"`
	IP       string `yaml:"ip,omitempty"`
}

var (
	ErrNoTestsDefined = fmt.Errorf("no tests defined")

	ErrHostMalformed   = fmt.Errorf("host is malformed")
	ErrInvalidHostName = fmt.Errorf("invalid hostname")
	ErrInvalidIp       = fmt.Errorf("invalid ip")
)

func (test ManifestTests) Validate() error {
	if test == nil {
		return ErrNoTestsDefined
	}

	hasAtLeastOne := false
	for _, t := range test {
		if t != nil {
			hasAtLeastOne = true
			break
		}
	}

	if !hasAtLeastOne {
		return ErrNoTestsDefined
	}

	return nil
}

func (test *Test) expandEnvVars() error {
	var err error
	if len(test.Image) > 0 {
		test.Image, err = vars.GlobalVarManager.ExpandExcLocal(test.Image)
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *Test) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type testAlias Test
	var tt testAlias
	err := unmarshal(&tt)
	if err != nil {
		return err
	}
	if tt.Context == "" {
		tt.Context = "."
	}
	*t = Test(tt)
	return nil
}

type TestCommand struct {
	Name    string `yaml:"name,omitempty"`
	Command string `yaml:"command,omitempty"`
}

func (t *TestCommand) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var command string
	err := unmarshal(&command)
	if err == nil {
		t.Command = command
		t.Name = command
		return nil
	}

	// prevent recursion
	type testCommandAlias TestCommand
	var extendedCommand testCommandAlias
	err = unmarshal(&extendedCommand)
	if err != nil {
		return err
	}
	*t = TestCommand(extendedCommand)
	return nil
}

func (h *Host) UnmarshalYAML(unmarshal func(interface{}) error) error {
	numberOfHostFields := 2
	var hostnameIP string
	err := unmarshal(&hostnameIP)
	if err == nil {
		hostnameIPExpanded, err := vars.GlobalVarManager.ExpandExcLocal(hostnameIP)
		if err != nil {
			return err
		}
		splittedHostNameIP := strings.SplitN(hostnameIPExpanded, ":", numberOfHostFields)
		if len(splittedHostNameIP) != numberOfHostFields {
			return fmt.Errorf("%w: '%s'", ErrHostMalformed, hostnameIP)
		}
		*h = Host{
			Hostname: splittedHostNameIP[0],
			IP:       splittedHostNameIP[1],
		}
		if h.Hostname == "" {
			return fmt.Errorf("%w: '%s' hostname is empty", ErrInvalidHostName, hostnameIP)
		}
		if h.IP == "" {
			return fmt.Errorf("%w: '%s' ip is empty", ErrInvalidIp, hostnameIP)
		}
		return nil
	}

	type hostAlias Host
	var hh hostAlias
	err = unmarshal(&hh)
	if err != nil {
		return err
	}

	hostname, err := vars.GlobalVarManager.ExpandExcLocal(hh.Hostname)
	if err != nil {
		return err
	}
	ip, err := vars.GlobalVarManager.ExpandExcLocal(hh.IP)
	if err != nil {
		return err
	}
	hh.Hostname = hostname
	hh.IP = ip
	if hh.Hostname == "" {
		return fmt.Errorf("%w: '%s' hostname is empty", ErrInvalidHostName, hostnameIP)
	}
	if hh.IP == "" {
		return fmt.Errorf("%w: '%s' ip is empty", ErrInvalidIp, hostnameIP)
	}

	*h = Host(hh)
	return nil
}
