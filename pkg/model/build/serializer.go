// Copyright 2021 The Okteto Authors
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

package build

import "github.com/okteto/okteto/pkg/model/environment"

// BuildInfoRaw represents the build info for serialization
type buildInfoRaw struct {
	Name       string                  `yaml:"name,omitempty"`
	Context    string                  `yaml:"context,omitempty"`
	Dockerfile string                  `yaml:"dockerfile,omitempty"`
	CacheFrom  []string                `yaml:"cache_from,omitempty"`
	Target     string                  `yaml:"target,omitempty"`
	Args       environment.Environment `yaml:"args,omitempty"`
	Image      string                  `yaml:"image,omitempty"`
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (buildInfo *Build) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rawString string
	err := unmarshal(&rawString)
	if err == nil {
		buildInfo.Name = rawString
		return nil
	}

	var rawBuildInfo buildInfoRaw
	err = unmarshal(&rawBuildInfo)
	if err != nil {
		return err
	}

	buildInfo.Name = rawBuildInfo.Name
	buildInfo.Context = rawBuildInfo.Context
	buildInfo.Dockerfile = rawBuildInfo.Dockerfile
	buildInfo.Target = rawBuildInfo.Target
	buildInfo.Args = rawBuildInfo.Args
	buildInfo.Image = rawBuildInfo.Image
	buildInfo.CacheFrom = rawBuildInfo.CacheFrom
	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (buildInfo Build) MarshalYAML() (interface{}, error) {
	if buildInfo.Context != "" && buildInfo.Context != "." {
		return buildInfoRaw(buildInfo), nil
	}
	if buildInfo.Dockerfile != "" && buildInfo.Dockerfile != "./Dockerfile" {
		return buildInfoRaw(buildInfo), nil
	}
	if buildInfo.Target != "" {
		return buildInfoRaw(buildInfo), nil
	}
	if buildInfo.Args != nil && len(buildInfo.Args) != 0 {
		return buildInfoRaw(buildInfo), nil
	}
	return buildInfo.Name, nil
}
