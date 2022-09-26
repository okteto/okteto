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
	"net/url"
	"strings"

	oktetoLog "github.com/okteto/okteto/pkg/log"
)

// ManifestDependencies represents the map of dependencies at a manifest
type ManifestDependencies map[string]Dependency

// GetRemoteDependencies returns a list of the remotes dependencies
func (md ManifestDependencies) GetRemoteDependencies() []RemoteDependency {
	result := []RemoteDependency{}
	for _, dependency := range md {
		remoteDependency, ok := dependency.(*RemoteDependency)
		if ok {
			result = append(result, *remoteDependency)
		}
	}
	return result
}

// GetLocalDependencies returns a list of the local dependencies
func (md ManifestDependencies) GetLocalDependencies() []LocalDependency {
	result := []LocalDependency{}
	for _, dependency := range md {
		localDependency, ok := dependency.(*LocalDependency)
		if ok {
			result = append(result, *localDependency)
		}
	}
	return result
}

// Dependency represents a dependency object at the manifest
type Dependency interface{}

// RemoteDependency represents a remote dependency object at the manifest
type RemoteDependency struct {
	Repository   string
	ManifestPath string
	Branch       string
	Variables    Environment
	Wait         bool
}

type localPath struct {
	absolutePath string
	relativePath string
}

// LocalDependency represents a remote dependency object at the manifest
type LocalDependency struct {
	manifestPath *localPath
	variables    Environment
}

type dependencyMarshaller struct {
	remoteDependency *RemoteDependency
	localDependency  *LocalDependency
	name             string
}

func (dm *dependencyMarshaller) toDependency() Dependency {
	if dm.localDependency != nil {
		return dm.localDependency
	}
	return dm.remoteDependency
}

func (dm *dependencyMarshaller) getName() string {
	if dm.localDependency != nil {
		return ""
	}
	repo, err := url.Parse(dm.remoteDependency.Repository)
	if err != nil {
		oktetoLog.Debugf("could not parse repo url: %w", err)
	}
	return getDependencyNameFromGitURL(repo)
}

type manifestDependenciesMarshaller map[string]*dependencyMarshaller

func (mdm manifestDependenciesMarshaller) toManifestDependencies() ManifestDependencies {
	result := ManifestDependencies{}
	for k, v := range mdm {
		result[k] = v.toDependency()
	}
	return result
}

type remoteDependencyMarshaller struct {
	Repository   string      `json:"repository" yaml:"repository"`
	ManifestPath string      `json:"manifest,omitempty" yaml:"manifest,omitempty"`
	Branch       string      `json:"branch,omitempty" yaml:"branch,omitempty"`
	Variables    Environment `json:"variables,omitempty" yaml:"variables,omitempty"`
	Wait         bool        `json:"wait,omitempty" yaml:"wait,omitempty"`
}

type localDependencyMarshaller struct {
	ManifestPath *localPath  `json:"manifest,omitempty" yaml:"manifest,omitempty"`
	Variables    Environment `json:"variables,omitempty" yaml:"variables,omitempty"`
}

func (ld *localDependencyMarshaller) toLocalDependency() *LocalDependency {
	return &LocalDependency{
		manifestPath: ld.ManifestPath,
		variables:    ld.Variables,
	}
}

// NewManifestDependenciesSection creates a new manifest dependencies section object
func NewManifestDependenciesSection() ManifestDependencies {
	return ManifestDependencies{}
}

func getDependencyNameFromGitURL(repo *url.URL) string {
	repoPath := strings.Split(strings.TrimPrefix(repo.Path, "/"), "/")
	return strings.ReplaceAll(repoPath[1], ".git", "")
}
