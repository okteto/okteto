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
	"os"
	"path/filepath"
	"strings"

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
	CacheFrom        cache.From        `yaml:"cache_from,omitempty"`
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
	CacheFrom        cache.From        `yaml:"cache_from,omitempty"`
	Args             Args              `yaml:"args,omitempty"`
	VolumesToInclude []VolumeMounts    `yaml:"-"`
	ExportCache      cache.ExportCache `yaml:"export_cache,omitempty"`
	DependsOn        DependsOn         `yaml:"depends_on,omitempty"`
}

func (i *Info) addExpandedPreviousImageArgs(previousImageArgs map[string]string) error {
	alreadyAddedArg := map[string]bool{}
	for _, arg := range i.Args {
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
		i.Args = append(i.Args, Arg{
			Name:  k,
			Value: expandedValue,
		})
		oktetoLog.Infof("Added '%s' to build args", k)
	}
	return nil
}

func (i *Info) expandManifestBuildArgs(previousImageArgs map[string]string) (err error) {
	for idx, arg := range i.Args {
		if val, ok := previousImageArgs[arg.Name]; ok {
			oktetoLog.Infof("overriding '%s' with the content of previous build", arg.Name)
			arg.Value = val
		}
		arg.Value, err = env.ExpandEnv(arg.Value)
		if err != nil {
			return err
		}
		i.Args[idx] = arg
	}
	return nil
}

func (i *Info) expandSecrets() (err error) {
	for k, v := range i.Secrets {
		val := v
		if strings.HasPrefix(val, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			val = filepath.Join(home, val[2:])
		}
		i.Secrets[k], err = env.ExpandEnv(val)
		if err != nil {
			return err
		}
	}
	return nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (i *Info) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rawString string
	err := unmarshal(&rawString)
	if err == nil {
		i.Name = rawString
		return nil
	}

	var rawBuildInfo infoRaw
	err = unmarshal(&rawBuildInfo)
	if err != nil {
		return err
	}

	i.Name = rawBuildInfo.Name
	i.Context = rawBuildInfo.Context
	i.Dockerfile = rawBuildInfo.Dockerfile
	i.Target = rawBuildInfo.Target
	i.Args = rawBuildInfo.Args
	i.Image = rawBuildInfo.Image
	i.CacheFrom = rawBuildInfo.CacheFrom
	i.ExportCache = rawBuildInfo.ExportCache
	i.DependsOn = rawBuildInfo.DependsOn
	i.Secrets = rawBuildInfo.Secrets
	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (i *Info) MarshalYAML() (interface{}, error) {
	if i.Context != "" && i.Context != "." {
		return infoRaw(*i), nil
	}
	if i.Dockerfile != "" && i.Dockerfile != "./Dockerfile" {
		return infoRaw(*i), nil
	}
	if i.Target != "" {
		return infoRaw(*i), nil
	}
	if i.Args != nil && len(i.Args) != 0 {
		return infoRaw(*i), nil
	}
	return i.Name, nil
}

// Copy clones the buildInfo without the pointers
func (i *Info) Copy() *Info {
	result := &Info{
		Name:        i.Name,
		Context:     i.Context,
		Dockerfile:  i.Dockerfile,
		Target:      i.Target,
		Image:       i.Image,
		ExportCache: i.ExportCache,
	}

	// copy to new pointers
	cacheFrom := []string{}
	cacheFrom = append(cacheFrom, i.CacheFrom...)
	result.CacheFrom = cacheFrom

	args := Args{}
	args = append(args, i.Args...)
	result.Args = args

	secrets := Secrets{}
	for k, v := range i.Secrets {
		secrets[k] = v
	}
	result.Secrets = secrets

	volumesToMount := []VolumeMounts{}
	volumesToMount = append(volumesToMount, i.VolumesToInclude...)
	result.VolumesToInclude = volumesToMount

	dependsOn := DependsOn{}
	dependsOn = append(dependsOn, i.DependsOn...)
	result.DependsOn = dependsOn

	return result
}

func (i *Info) SetBuildDefaults() {
	if i.Context == "" {
		i.Context = "."
	}

	if _, err := url.ParseRequestURI(i.Context); err != nil && i.Dockerfile == "" {
		i.Dockerfile = "Dockerfile"
	}

}

// AddArgs add a set of args to the build information
func (i *Info) AddArgs(previousImageArgs map[string]string) error {
	if err := i.expandManifestBuildArgs(previousImageArgs); err != nil {
		return err
	}
	if err := i.expandSecrets(); err != nil {
		return err
	}
	return i.addExpandedPreviousImageArgs(previousImageArgs)
}

// GetDockerfilePath returns the path to the Dockerfile
func (i *Info) GetDockerfilePath(fs afero.Fs) string {
	if filepath.IsAbs(i.Dockerfile) {
		return i.Dockerfile
	}
	joinPath := filepath.Join(i.Context, i.Dockerfile)
	if !filesystem.FileExistsAndNotDir(joinPath, fs) {
		oktetoLog.Infof("Dockerfile '%s' is not in a relative path to context '%s'", i.Dockerfile, i.Context)
		return i.Dockerfile
	}

	if joinPath != filepath.Clean(i.Dockerfile) && filesystem.FileExistsAndNotDir(i.Dockerfile, fs) {
		oktetoLog.Infof("Two Dockerfiles discovered in both the root and context path, defaulting to '%s/%s'", i.Context, i.Dockerfile)
	}

	return joinPath
}
