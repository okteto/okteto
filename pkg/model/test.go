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

	"github.com/okteto/okteto/pkg/env"
)

type Test struct {
	Image     string        `yaml:"image,omitempty"`
	Context   string        `yaml:"context,omitempty"`
	Commands  []TestCommand `yaml:"commands,omitempty"`
	DependsOn []string      `yaml:"depends_on,omitempty"`
	Caches    []string      `yaml:"caches,omitempty"`
	Artifacts []Artifact    `yaml:"artifacts,omitempty"`
}

var (
	ErrNoTestsDefined = fmt.Errorf("no tests defined")
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
		test.Image, err = env.ExpandEnv(test.Image)
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
