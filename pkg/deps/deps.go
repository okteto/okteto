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

package deps

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/a8m/envsubst/parse"
	giturls "github.com/chainguard-dev/git-urls"
	"github.com/okteto/okteto/pkg/env"
	"github.com/okteto/okteto/pkg/model/utils"
	"github.com/okteto/okteto/pkg/vars"
)

// ManifestSection represents the map of dependencies at a manifest
type ManifestSection map[string]*Dependency

// Dependency represents a dependency object at the manifest
type Dependency struct {
	Repository   string          `json:"repository" yaml:"repository"`
	ManifestPath string          `json:"manifest,omitempty" yaml:"manifest,omitempty"`
	Branch       string          `json:"branch,omitempty" yaml:"branch,omitempty"`
	Variables    env.Environment `json:"variables,omitempty" yaml:"variables,omitempty"`
	Timeout      time.Duration   `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Wait         bool            `json:"wait,omitempty" yaml:"wait,omitempty"`
}

// GetTimeout returns dependency.Timeout if it's set or the one passed as arg if it's not
func (d *Dependency) GetTimeout(defaultTimeout time.Duration) time.Duration {
	if d.Timeout != 0 {
		return d.Timeout
	}
	return defaultTimeout
}

// ExpandVars sets dependencies values if values fits with list params
func (d *Dependency) ExpandVars(variables []string, varManager *vars.Manager) error {
	parser := parse.New("string", append(varManager.GetOktetoVariablesExcLocal(), variables...), &parse.Restrictions{})

	expandedBranch, err := parser.Parse(d.Branch)
	if err != nil {
		return fmt.Errorf("error expanding 'branch': %w", err)
	}
	if expandedBranch != "" {
		d.Branch = expandedBranch
	}

	expandedRepository, err := parser.Parse(d.Repository)
	if err != nil {
		return fmt.Errorf("error expanding 'repository': %w", err)
	}
	if expandedRepository != "" {
		d.Repository = expandedRepository
	}

	expandedManifestPath, err := parser.Parse(d.ManifestPath)
	if err != nil {
		return fmt.Errorf("error expanding 'manifest': %w", err)
	}
	if expandedManifestPath != "" {
		d.ManifestPath = expandedManifestPath
	}

	expandedVariables := env.Environment{}
	for _, v := range d.Variables {
		expandedVarName, err := parser.Parse(v.Name)
		if err != nil {
			return fmt.Errorf("error expanding variable name: %w", err)
		}
		if expandedVarName != "" {
			v.Name = expandedVarName
		}

		expandedVarValue, err := parser.Parse(v.Value)
		if err != nil {
			return fmt.Errorf("error expanding variable value: %w", err)
		}
		if expandedVarValue != "" {
			v.Value = expandedVarValue
		}

		expandedVariables = append(expandedVariables, vars.Var{
			Name:  v.Name,
			Value: v.Value,
		})
	}
	d.Variables = expandedVariables

	return nil
}

func getRepoNameFromGitURL(repo *url.URL) (string, error) {
	repoPath := strings.Split(strings.TrimPrefix(repo.Path, "/"), "/")
	if len(repoPath) < 2 || repoPath[1] == "" {
		return "", fmt.Errorf("dependency has invalid repository url: %s", repo.String())
	}
	return strings.ReplaceAll(repoPath[1], ".git", ""), nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (md *ManifestSection) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rawList []string
	err := unmarshal(&rawList)
	if err == nil {
		rawMd := ManifestSection{}
		for _, repo := range rawList {
			r, err := giturls.Parse(repo)
			if err != nil {
				return err
			}
			name := utils.TranslateURLToName(r.String())
			rawMd[name] = &Dependency{
				Repository: r.String(),
			}
		}
		*md = rawMd
		return nil
	}

	type manifestDependencies ManifestSection
	var rawMap manifestDependencies
	err = unmarshal(&rawMap)
	if err != nil {
		return err
	}
	*md = ManifestSection(rawMap)
	return nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (d *Dependency) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rawString string
	err := unmarshal(&rawString)
	if err == nil {
		d.Repository = rawString
		return nil
	}

	type dependencyPreventRecursionType Dependency
	var dependencyRaw dependencyPreventRecursionType
	err = unmarshal(&dependencyRaw)
	if err != nil {
		return err
	}
	*d = Dependency(dependencyRaw)

	return nil
}

func (md ManifestSection) IsEmpty() bool {
	return len(md) == 0
}
