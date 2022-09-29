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
	"fmt"
	"net/url"
	"strings"

	"github.com/okteto/okteto/pkg/filesystem"
	oktetoLog "github.com/okteto/okteto/pkg/log"
)

// ManifestDependencies represents the map of dependencies at a manifest
type ManifestDependencies map[string]Dependency

func (ManifestDependencies) merge(other ManifestDependencies) []string {
	warnings := []string{}
	if len(other) > 0 {
		warnings = append(warnings, "dependencies: dependencies are only supported on the main manifest")
	}
	return warnings
}

// GetRemoteDependencies returns a map of the remotes dependencies
func (md ManifestDependencies) GetRemoteDependencies() map[string]*RemoteDependency {
	result := map[string]*RemoteDependency{}
	for dependencyName, dependency := range md {
		remoteDependency, ok := dependency.(*RemoteDependency)
		if ok {
			result[dependencyName] = remoteDependency
		}
	}
	return result
}

// GetLocalDependencies returns a list of the local dependencies
func (md ManifestDependencies) GetLocalDependencies() map[string]*LocalDependency {
	result := map[string]*LocalDependency{}
	for dependencyName, dependency := range md {
		localDependency, ok := dependency.(*LocalDependency)
		if ok {
			result[dependencyName] = localDependency
		}
	}
	return result
}

// GetRemoteDependencies returns a map of the remotes dependencies
func (md ManifestDependencies) validate() error {
	for dependencyName, dependencyInfo := range md.GetLocalDependencies() {
		dependencyPath := dependencyInfo.GetManifestPath()
		if !filesystem.FileExists(dependencyPath) {
			return fmt.Errorf("dependency '%s' not found", dependencyName)
		}
		if !filesystem.FileExistsAndNotDir(dependencyPath) {
			return fmt.Errorf("dependency '%s' is a directory", dependencyName)
		}
	}
	return nil
}

// Dependency represents a dependency object at the manifest
type Dependency interface{}

// RemoteDependency represents a remote dependency object at the manifest
type RemoteDependency struct {
	repository   string
	manifestPath string
	branch       string
	variables    Environment
	wait         bool
}

// NewRemoteDependencyFromRepository returns a remote dependency from a repository
func NewRemoteDependencyFromRepository(repository string) *RemoteDependency {
	return &RemoteDependency{
		repository: repository,
	}
}

// GetRepository returns the repository of the dependency
func (rd *RemoteDependency) GetRepository() string {
	return rd.repository
}

// GetBranch returns the branch of the dependency
func (rd *RemoteDependency) GetBranch() string {
	return rd.branch
}

// GetManifestPath returns the manifest path of the dependency
func (rd *RemoteDependency) GetManifestPath() string {
	return rd.manifestPath
}

// HasToWait returns if Okteto should wait until the execution is finished
func (rd *RemoteDependency) HasToWait() bool {
	return rd.wait
}

// GetVariables returns the variables of the dependency
func (rd *RemoteDependency) GetVariables() Environment {
	return rd.variables
}

// AddVariable adds a variable to the dependency variable list
func (rd *RemoteDependency) AddVariable(key, value string) {
	rd.variables = append(rd.variables, EnvVar{
		Name:  key,
		Value: value,
	})
}

// LocalDependency represents a remote dependency object at the manifest
type LocalDependency struct {
	manifestPath string
}

// GetManifestPath returns the manifest path of the dependency
func (rd *LocalDependency) GetManifestPath() string {
	return rd.manifestPath
}

type dependencyMarshaller struct {
	remoteDependency *RemoteDependency
	localDependency  *LocalDependency
}

func (dm *dependencyMarshaller) toDependency() Dependency {
	if dm.localDependency != nil {
		return dm.localDependency
	}
	return dm.remoteDependency
}

func (dm *dependencyMarshaller) getName() string {
	if dm.localDependency != nil {
		return dm.localDependency.manifestPath
	}
	repo, err := url.Parse(dm.remoteDependency.GetRepository())
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

func (ld *remoteDependencyMarshaller) toRemoteDependency() *RemoteDependency {
	return &RemoteDependency{
		repository:   ld.Repository,
		manifestPath: ld.ManifestPath,
		branch:       ld.Branch,
		variables:    ld.Variables,
		wait:         ld.Wait,
	}
}

type localDependencyMarshaller struct {
	ManifestPath string `json:"manifest,omitempty" yaml:"manifest,omitempty"`
}

func (ld *localDependencyMarshaller) toLocalDependency() *LocalDependency {
	return &LocalDependency{
		manifestPath: ld.ManifestPath,
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
