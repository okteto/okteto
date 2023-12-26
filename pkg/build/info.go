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
	"net/url"
	"path/filepath"

	"github.com/okteto/okteto/pkg/cache"
	"github.com/okteto/okteto/pkg/env"
	"github.com/okteto/okteto/pkg/filesystem"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/spf13/afero"
)

// Info represents the build info to generate an image
type Info struct {
	Secrets          Secrets           `yaml:"secrets,omitempty"`
	Name             string            `yaml:"name,omitempty"`
	Context          string            `yaml:"context,omitempty"`
	Dockerfile       string            `yaml:"dockerfile,omitempty"`
	Target           string            `yaml:"target,omitempty"`
	Image            string            `yaml:"image,omitempty"`
	CacheFrom        cache.CacheFrom   `yaml:"cache_from,omitempty"`
	Args             Args              `yaml:"args,omitempty"`
	VolumesToInclude []VolumeMounts    `yaml:"-"`
	ExportCache      cache.ExportCache `yaml:"export_cache,omitempty"`
	DependsOn        DependsOn         `yaml:"depends_on,omitempty"`
}

// Secrets represents the secrets to be injected to the build of the image
type Secrets map[string]string

// infoRaw represents the build info for serialization
type infoRaw struct {
	Secrets          Secrets           `yaml:"secrets,omitempty"`
	Name             string            `yaml:"name,omitempty"`
	Context          string            `yaml:"context,omitempty"`
	Dockerfile       string            `yaml:"dockerfile,omitempty"`
	Target           string            `yaml:"target,omitempty"`
	Image            string            `yaml:"image,omitempty"`
	CacheFrom        cache.CacheFrom   `yaml:"cache_from,omitempty"`
	Args             Args              `yaml:"args,omitempty"`
	VolumesToInclude []VolumeMounts    `yaml:"-"`
	ExportCache      cache.ExportCache `yaml:"export_cache,omitempty"`
	DependsOn        DependsOn         `yaml:"depends_on,omitempty"`
}

func (b *Info) addExpandedPreviousImageArgs(previousImageArgs map[string]string) error {
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

func (b *Info) expandManifestBuildArgs(previousImageArgs map[string]string) (err error) {
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

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (info *Info) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rawString string
	err := unmarshal(&rawString)
	if err == nil {
		info.Name = rawString
		return nil
	}

	var rawBuildInfo infoRaw
	err = unmarshal(&rawBuildInfo)
	if err != nil {
		return err
	}

	info.Name = rawBuildInfo.Name
	info.Context = rawBuildInfo.Context
	info.Dockerfile = rawBuildInfo.Dockerfile
	info.Target = rawBuildInfo.Target
	info.Args = rawBuildInfo.Args
	info.Image = rawBuildInfo.Image
	info.CacheFrom = rawBuildInfo.CacheFrom
	info.ExportCache = rawBuildInfo.ExportCache
	info.DependsOn = rawBuildInfo.DependsOn
	info.Secrets = rawBuildInfo.Secrets
	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (info *Info) MarshalYAML() (interface{}, error) {
	if info.Context != "" && info.Context != "." {
		return infoRaw(*info), nil
	}
	if info.Dockerfile != "" && info.Dockerfile != "./Dockerfile" {
		return infoRaw(*info), nil
	}
	if info.Target != "" {
		return infoRaw(*info), nil
	}
	if info.Args != nil && len(info.Args) != 0 {
		return infoRaw(*info), nil
	}
	return info.Name, nil
}

// Copy clones the buildInfo without the pointers
func (b *Info) Copy() *Info {
	result := &Info{
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

	secrets := Secrets{}
	for k, v := range b.Secrets {
		secrets[k] = v
	}
	result.Secrets = secrets

	volumesToMount := []VolumeMounts{}
	volumesToMount = append(volumesToMount, b.VolumesToInclude...)
	result.VolumesToInclude = volumesToMount

	dependsOn := DependsOn{}
	dependsOn = append(dependsOn, b.DependsOn...)
	result.DependsOn = dependsOn

	return result
}

func (b *Info) SetBuildDefaults() {
	if b.Context == "" {
		b.Context = "."
	}

	if _, err := url.ParseRequestURI(b.Context); err != nil && b.Dockerfile == "" {
		b.Dockerfile = "Dockerfile"
	}

}

// AddArgs add a set of args to the build information
func (b *Info) AddArgs(previousImageArgs map[string]string) error {
	if err := b.expandManifestBuildArgs(previousImageArgs); err != nil {
		return err
	}
	return b.addExpandedPreviousImageArgs(previousImageArgs)
}

// GetDockerfilePath returns the path to the Dockerfile
func (b *Info) GetDockerfilePath(fs afero.Fs) string {
	if filepath.IsAbs(b.Dockerfile) {
		return b.Dockerfile
	}
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
