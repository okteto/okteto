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

type Artifact struct {
	Path        string `yaml:"path,omitempty"`
	Destination string `yaml:"destination,omitempty"`
}

func (t *Artifact) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var path string
	err := unmarshal(&path)
	if err == nil {
		t.Path = path
		t.Destination = path
		return nil
	}

	// prevent recursion
	type artifactAlias Artifact
	var extendedArtifact artifactAlias
	err = unmarshal(&extendedArtifact)
	if err != nil {
		return err
	}
	if extendedArtifact.Destination == "" {
		extendedArtifact.Destination = extendedArtifact.Path
	}
	*t = Artifact(extendedArtifact)
	return nil
}
