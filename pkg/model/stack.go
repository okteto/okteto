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
	"errors"
	"fmt"
	"io/ioutil"
	"strings"

	yaml "gopkg.in/yaml.v2"
)

var (
	errBadStackName = "must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character"
)

//Stack represents an okteto stack
type Stack struct {
	Name      string             `yaml:"name"`
	Namespace string             `yaml:"namespace,omitempty"`
	Services  map[string]Service `yaml:"services,omitempty"`
}

//Service represents an okteto stack service
type Service struct {
	Public          bool         `yaml:"public,omitempty"`
	Image           string       `yaml:"image"`
	Build           string       `yaml:"build,omitempty"`
	Replicas        int          `yaml:"replicas"`
	Command         string       `yaml:"command,omitempty"`
	Environment     []EnvVar     `yaml:"environment,omitempty"`
	Ports           []int        `yaml:"ports,omitempty"`
	Volumes         []string     `yaml:"volumes,omitempty"`
	StopGracePeriod int          `yaml:"stop_grace_period,omitempty"`
	Resources       ResourceList `yaml:"resources,omitempty"`
}

//GetStack returns an okteto stack object from a given file
func GetStack(name, stackPath string) (*Stack, error) {
	b, err := ioutil.ReadFile(stackPath)
	if err != nil {
		return nil, err
	}

	s, err := ReadStack(b)
	if err != nil {
		return nil, err
	}

	if name != "" {
		s.Name = name
	}
	if err := s.validate(); err != nil {
		return nil, err
	}

	return s, nil
}

//ReadStack reads an okteto stack
func ReadStack(bytes []byte) (*Stack, error) {
	s := &Stack{}
	if err := yaml.UnmarshalStrict(bytes, s); err != nil {
		if strings.HasPrefix(err.Error(), "yaml: unmarshal errors:") {
			var sb strings.Builder
			_, _ = sb.WriteString("Invalid stack manifest:\n")
			l := strings.Split(err.Error(), "\n")
			for i := 1; i < len(l); i++ {
				e := strings.TrimSuffix(l[i], "in type model.Stack")
				e = strings.TrimSpace(e)
				_, _ = sb.WriteString(fmt.Sprintf("    - %s\n", e))
			}

			_, _ = sb.WriteString("    See https://okteto.com/docs/reference/stack for details")
			return nil, errors.New(sb.String())
		}

		msg := strings.Replace(err.Error(), "yaml: unmarshal errors:", "invalid stack manifest:", 1)
		msg = strings.TrimSuffix(msg, "in type model.Stack")
		return nil, errors.New(msg)
	}
	for i, svc := range s.Services {
		if svc.Replicas == 0 {
			svc.Replicas = 1
			s.Services[i] = svc
		}
	}
	return s, nil
}

func (s *Stack) validate() error {
	if err := validateStackName(s.Name); err != nil {
		return fmt.Errorf("Invalid stack name: %s", err)
	}
	if len(s.Services) == 0 {
		return fmt.Errorf("Invalid stack: 'services' cannot be empty")
	}

	for name, svc := range s.Services {
		if err := validateStackName(name); err != nil {
			return fmt.Errorf("Invalid service name '%s': %s", name, err)
		}
		if svc.Image == "" {
			return fmt.Errorf(fmt.Sprintf("Invalid service '%s': image cannot be empty", name))
		}
		for _, v := range svc.Volumes {
			if !strings.HasPrefix(v, "/") {
				return fmt.Errorf(fmt.Sprintf("Invalid volume '%s' in service '%s': must be an absolute path", v, name))
			}
			if strings.Contains(v, ":") {
				return fmt.Errorf(fmt.Sprintf("Invalid volume '%s' in service '%s': volume bind mounts are not supported", v, name))
			}
		}
	}

	return nil
}

func validateStackName(name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if ValidKubeNameRegex.MatchString(name) {
		return fmt.Errorf(errBadStackName)
	}
	if strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") {
		return fmt.Errorf(errBadStackName)
	}
	return nil
}

//UpdateNamespace updates the dev namespace
func (s *Stack) UpdateNamespace(namespace string) error {
	if namespace == "" {
		return nil
	}
	if s.Namespace != "" && s.Namespace != namespace {
		return fmt.Errorf("the namespace in the okteto stack manifest '%s' does not match the namespace '%s'", s.Namespace, namespace)
	}
	s.Namespace = namespace
	return nil
}
