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
	"net/url"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/okteto/okteto/pkg/cache"
	"github.com/okteto/okteto/pkg/env"
	"github.com/okteto/okteto/pkg/filesystem"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model/utils"
	"github.com/spf13/afero"
)

// BuildInfo represents the build info to generate an image
type BuildInfo struct {
	Secrets          BuildSecrets      `yaml:"secrets,omitempty"`
	Name             string            `yaml:"name,omitempty"`
	Context          string            `yaml:"context,omitempty"`
	Dockerfile       string            `yaml:"dockerfile,omitempty"`
	Target           string            `yaml:"target,omitempty"`
	Image            string            `yaml:"image,omitempty"`
	CacheFrom        cache.CacheFrom   `yaml:"cache_from,omitempty"`
	Args             Args              `yaml:"args,omitempty"`
	VolumesToInclude []VolumeMounts    `yaml:"-"`
	ExportCache      cache.ExportCache `yaml:"export_cache,omitempty"`
	DependsOn        BuildDependsOn    `yaml:"depends_on,omitempty"`
}

type VolumeMounts struct {
	LocalPath  string
	RemotePath string
}

// BuildDependsOn represents the images that needs to be built before
type BuildDependsOn []string

// BuildSecrets represents the secrets to be injected to the build of the image
type BuildSecrets map[string]string

// ManifestBuild defines all the build section
type ManifestBuild map[string]*BuildInfo

// GetDockerfilePath returns the path to the Dockerfile
func (b *BuildInfo) GetDockerfilePath() string {
	if filepath.IsAbs(b.Dockerfile) {
		return b.Dockerfile
	}
	fs := afero.NewOsFs()
	joinPath := filepath.Join(b.Context, b.Dockerfile)
	if !filesystem.FileExistsAndNotDir(joinPath, fs) {
		oktetoLog.Infof("Dockerfile '%s' is not in a relative path to context '%s'", b.Dockerfile, b.Context)
		return b.Dockerfile
	}

	if joinPath != filepath.Clean(b.Dockerfile) && filesystem.FileExistsAndNotDir(b.Dockerfile, fs) {
		oktetoLog.Infof("Two Dockerfiles discovered in both the root and context path, defaulting to '%s/%s'", b.Context, b.Dockerfile)
	}

	return joinPath
}

// AddBuildArgs add a set of args to the build information
func (b *BuildInfo) AddBuildArgs(previousImageArgs map[string]string) error {
	if err := b.expandManifestBuildArgs(previousImageArgs); err != nil {
		return err
	}
	return b.addExpandedPreviousImageArgs(previousImageArgs)
}

func (b *BuildInfo) expandManifestBuildArgs(previousImageArgs map[string]string) (err error) {
	for idx, arg := range b.Args {
		if val, ok := previousImageArgs[arg.Name]; ok {
			oktetoLog.Infof("overriding '%s' with the content of previous build", arg.Name)
			arg.Value = val
		}
		arg.Value, err = env.ExpandEnv(arg.Value)
		if err != nil {
			return err
		}
		b.Args[idx] = arg
	}
	return nil
}

func (b *BuildInfo) addExpandedPreviousImageArgs(previousImageArgs map[string]string) error {
	alreadyAddedArg := map[string]bool{}
	for _, arg := range b.Args {
		alreadyAddedArg[arg.Name] = true
	}
	for k, v := range previousImageArgs {
		if _, ok := alreadyAddedArg[k]; ok {
			continue
		}
		expandedValue, err := env.ExpandEnv(v)
		if err != nil {
			return err
		}
		b.Args = append(b.Args, Arg{
			Name:  k,
			Value: expandedValue,
		})
		oktetoLog.Infof("Added '%s' to build args", k)
	}
	return nil
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

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (buildInfo *BuildInfo) UnmarshalYAML(unmarshal func(interface{}) error) error {
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
	buildInfo.ExportCache = rawBuildInfo.ExportCache
	buildInfo.DependsOn = rawBuildInfo.DependsOn
	buildInfo.Secrets = rawBuildInfo.Secrets
	return nil
}

// BuildInfoRaw represents the build info for serialization
type buildInfoRaw struct {
	Secrets          BuildSecrets      `yaml:"secrets,omitempty"`
	Name             string            `yaml:"name,omitempty"`
	Context          string            `yaml:"context,omitempty"`
	Dockerfile       string            `yaml:"dockerfile,omitempty"`
	Target           string            `yaml:"target,omitempty"`
	Image            string            `yaml:"image,omitempty"`
	CacheFrom        cache.CacheFrom   `yaml:"cache_from,omitempty"`
	Args             Args              `yaml:"args,omitempty"`
	VolumesToInclude []VolumeMounts    `yaml:"-"`
	ExportCache      cache.ExportCache `yaml:"export_cache,omitempty"`
	DependsOn        BuildDependsOn    `yaml:"depends_on,omitempty"`
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (buildInfo *BuildInfo) MarshalYAML() (interface{}, error) {
	if buildInfo.Context != "" && buildInfo.Context != "." {
		return buildInfoRaw(*buildInfo), nil
	}
	if buildInfo.Dockerfile != "" && buildInfo.Dockerfile != "./Dockerfile" {
		return buildInfoRaw(*buildInfo), nil
	}
	if buildInfo.Target != "" {
		return buildInfoRaw(*buildInfo), nil
	}
	if buildInfo.Args != nil && len(buildInfo.Args) != 0 {
		return buildInfoRaw(*buildInfo), nil
	}
	return buildInfo.Name, nil
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

// Copy clones the buildInfo without the pointers
func (b *BuildInfo) Copy() *BuildInfo {
	result := &BuildInfo{
		Name:        b.Name,
		Context:     b.Context,
		Dockerfile:  b.Dockerfile,
		Target:      b.Target,
		Image:       b.Image,
		ExportCache: b.ExportCache,
	}

	// copy to new pointers
	cacheFrom := []string{}
	cacheFrom = append(cacheFrom, b.CacheFrom...)
	result.CacheFrom = cacheFrom

	args := Args{}
	args = append(args, b.Args...)
	result.Args = args

	secrets := BuildSecrets{}
	for k, v := range b.Secrets {
		secrets[k] = v
	}
	result.Secrets = secrets

	volumesToMount := []VolumeMounts{}
	volumesToMount = append(volumesToMount, b.VolumesToInclude...)
	result.VolumesToInclude = volumesToMount

	dependsOn := BuildDependsOn{}
	dependsOn = append(dependsOn, b.DependsOn...)
	result.DependsOn = dependsOn

	return result
}

func (b *BuildInfo) SetBuildDefaults() {
	if b.Context == "" {
		b.Context = "."
	}

	if _, err := url.ParseRequestURI(b.Context); err != nil && b.Dockerfile == "" {
		b.Dockerfile = "Dockerfile"
	}

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
