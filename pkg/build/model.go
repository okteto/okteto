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

package build

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model/utils"
)

type VolumeMounts struct {
	LocalPath  string
	RemotePath string
}

// BuildDependsOn represents the images that needs to be built before
type BuildDependsOn []string

// ManifestBuild defines all the build section
type ManifestBuild map[string]*Info

// AddBuildArgs add a set of args to the build information
func (b *Info) AddBuildArgs(previousImageArgs map[string]string) error {
	if err := b.expandManifestBuildArgs(previousImageArgs); err != nil {
		return err
	}
	return b.addExpandedPreviousImageArgs(previousImageArgs)
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (d *BuildDependsOn) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rawString string
	err := unmarshal(&rawString)
	if err == nil {
		*d = BuildDependsOn{rawString}
		return nil
	}

	var rawStringList []string
	err = unmarshal(&rawStringList)
	if err == nil {
		*d = rawStringList
		return nil
	}
	return err
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (v VolumeMounts) MarshalYAML() (interface{}, error) {
	return v.RemotePath, nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (v *VolumeMounts) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw string
	err := unmarshal(&raw)
	if err != nil {
		return err
	}

	stackVolumePartsOnlyRemote := 1
	stackVolumeParts := 2
	stackVolumeMaxParts := 3

	parts := strings.Split(raw, ":")
	if runtime.GOOS == "windows" {
		if len(parts) >= stackVolumeMaxParts {
			localPath := fmt.Sprintf("%s:%s", parts[0], parts[1])
			if filepath.IsAbs(localPath) {
				parts = append([]string{localPath}, parts[2:]...)
			}
		}
	}

	if len(parts) == stackVolumeParts {
		v.LocalPath = parts[0]
		v.RemotePath = parts[1]
	} else if len(parts) == stackVolumePartsOnlyRemote {
		v.RemotePath = parts[0]
	} else {
		return fmt.Errorf("Syntax error volumes should be 'local_path:remote_path' or 'remote_path'")
	}

	return nil
}

// ToString returns volume as string
func (v VolumeMounts) ToString() string {
	if v.LocalPath != "" {
		return fmt.Sprintf("%s:%s", v.LocalPath, v.RemotePath)
	}
	return v.RemotePath
}

func (b *ManifestBuild) Validate() error {
	for k, v := range *b {
		if v == nil {
			return fmt.Errorf("manifest validation failed: service '%s' build section not defined correctly", k)
		}
	}

	cycle := utils.GetDependentCyclic(b.toGraph())
	if len(cycle) == 1 { // depends on the same node
		return fmt.Errorf("manifest build validation failed: image '%s' is referenced on its dependencies", cycle[0])
	} else if len(cycle) > 1 {
		svcsDependents := fmt.Sprintf("%s and %s", strings.Join(cycle[:len(cycle)-1], ", "), cycle[len(cycle)-1])
		return fmt.Errorf("manifest validation failed: cyclic dependendecy found between %s", svcsDependents)
	}
	return nil
}

// GetSvcsToBuildFromList returns the builds from a list and all its
func (b *ManifestBuild) GetSvcsToBuildFromList(toBuild []string) []string {
	initialSvcsToBuild := toBuild
	svcsToBuildWithDependencies := utils.GetDependentNodes(b.toGraph(), toBuild)
	if len(initialSvcsToBuild) != len(svcsToBuildWithDependencies) {
		dependantBuildImages := utils.GetListDiff(initialSvcsToBuild, svcsToBuildWithDependencies)
		oktetoLog.Warning("The following build images need to be built because of dependencies: [%s]", strings.Join(dependantBuildImages, ", "))
	}
	return svcsToBuildWithDependencies
}

func (b ManifestBuild) toGraph() utils.Graph {
	g := utils.Graph{}
	for k, v := range b {
		g[k] = v.DependsOn
	}
	return g
}
