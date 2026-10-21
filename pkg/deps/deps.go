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
	"maps"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/a8m/envsubst/parse"
	giturls "github.com/chainguard-dev/git-urls"
	"github.com/okteto/okteto/pkg/env"
	"github.com/okteto/okteto/pkg/format"
	"github.com/okteto/okteto/pkg/model/utils"
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
	DependsOn    DependsOn       `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
}

// DependsOn represents the dependencies that need to be deployed successfully before
type DependsOn []string

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (d *DependsOn) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rawString string
	err := unmarshal(&rawString)
	if err == nil {
		*d = DependsOn{rawString}
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

// GetTimeout returns dependency.Timeout if it's set or the one passed as arg if it's not
func (d *Dependency) GetTimeout(defaultTimeout time.Duration) time.Duration {
	if d.Timeout != 0 {
		return d.Timeout
	}
	return defaultTimeout
}

// ExpandVars sets dependencies values if values fit with list params
func (d *Dependency) ExpandVars(variables []string) error {
	return d.ExpandVarsFrom(os.Environ(), variables)
}

// ExpandVarsFrom sets dependencies values expanding them with environ and the variables
func (d *Dependency) ExpandVarsFrom(environ, variables []string) error {
	parser := parse.New("string", append(append([]string{}, environ...), variables...), &parse.Restrictions{})

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

		// the values are not expanded when the manifest is read (see unexpandedVariables), so the first pass is the
		// expansion they used to get then, now with the env vars of the dependencies in depends_on. The second one
		// is the expansion they already had here, which resolves the escaped $${VAR}
		value, err := parser.Parse(v.Value)
		if err != nil {
			return fmt.Errorf("error expanding variable value: %w", err)
		}
		v.Value = value
		expandedVarValue, err := parser.Parse(value)
		if err != nil {
			return fmt.Errorf("error expanding variable value: %w", err)
		}
		if expandedVarValue != "" {
			v.Value = expandedVarValue
		}

		expandedVariables = append(expandedVariables, env.Var{
			Name:  v.Name,
			Value: v.Value,
		})
	}
	d.Variables = expandedVariables

	return nil
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
			if _, ok := rawMd[name]; ok {
				return fmt.Errorf("dependency '%s' is defined more than once", name)
			}
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

// dependencyFields are the fields of a dependency in the manifest. They are the ones of Dependency, except that
// the variables are read without expanding them
type dependencyFields struct {
	Repository   string              `yaml:"repository"`
	ManifestPath string              `yaml:"manifest,omitempty"`
	Branch       string              `yaml:"branch,omitempty"`
	Variables    unexpandedVariables `yaml:"variables,omitempty"`
	Timeout      time.Duration       `yaml:"timeout,omitempty"`
	Wait         bool                `yaml:"wait,omitempty"`
	DependsOn    DependsOn           `yaml:"depends_on,omitempty"`
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (d *Dependency) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rawString string
	err := unmarshal(&rawString)
	if err == nil {
		d.Repository = rawString
		return nil
	}

	var fields dependencyFields
	err = unmarshal(&fields)
	if err != nil {
		return err
	}
	*d = Dependency{
		Repository:   fields.Repository,
		ManifestPath: fields.ManifestPath,
		Branch:       fields.Branch,
		Variables:    env.Environment(fields.Variables),
		Timeout:      fields.Timeout,
		Wait:         fields.Wait,
		DependsOn:    fields.DependsOn,
	}

	return nil
}

// unexpandedVariables are the variables of a dependency as they are written in the manifest. Unlike the rest of
// the manifest, they are not expanded when it is read: the env vars of the dependencies in depends_on don't exist
// yet. ExpandVarsFrom expands them when the dependency is deployed
type unexpandedVariables env.Environment

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (v *unexpandedVariables) UnmarshalYAML(unmarshal func(interface{}) error) error {
	values := map[string]string{}
	var rawList []string
	if err := unmarshal(&rawList); err == nil {
		for _, raw := range rawList {
			name, value, found := strings.Cut(raw, "=")
			if !found {
				// a variable without value takes it from the environment, as in the rest of the manifest
				name, err = env.ExpandEnv(raw)
				if err != nil {
					return err
				}
				value = os.Getenv(name)
			}
			values[name] = value
		}
	} else {
		var rawMap map[string]string
		if err := unmarshal(&rawMap); err != nil {
			return err
		}
		maps.Copy(values, rawMap)
	}

	variables := make(unexpandedVariables, 0, len(values))
	for name, value := range values {
		variables = append(variables, env.Var{Name: name, Value: value})
	}
	sort.Slice(variables, func(i, j int) bool {
		return variables[i].Name < variables[j].Name
	})
	*v = variables
	return nil
}

// Validate validates the execution graph defined by the depends_on fields
func (md ManifestSection) Validate() error {
	names := make([]string, 0, len(md))
	for name := range md {
		names = append(names, name)
	}
	sort.Strings(names)

	// every dependency is deployed as the pipeline of its sanitized name
	pipelineNames := map[string]string{}
	for _, name := range names {
		pipelineName := format.ResourceK8sMetaString(name)
		if other, ok := pipelineNames[pipelineName]; ok {
			return fmt.Errorf("manifest validation failed: dependencies '%s' and '%s' resolve to the same name '%s'", other, name, pipelineName)
		}
		pipelineNames[pipelineName] = name
	}

	for _, name := range names {
		dep := md[name]
		if dep == nil {
			return fmt.Errorf("manifest validation failed: dependency '%s' is not defined correctly", name)
		}
		for _, predecessor := range dep.DependsOn {
			if predecessor == name {
				return fmt.Errorf("manifest validation failed: dependency '%s' cannot depend on itself", name)
			}
			if _, ok := md[predecessor]; !ok {
				return fmt.Errorf("manifest validation failed: dependency '%s' depends on '%s', which is not defined in the dependencies section", name, predecessor)
			}
		}
	}

	cycle := utils.GetDependentCyclic(md.toGraph())
	if len(cycle) > 1 {
		sort.Strings(cycle)
		depsInCycle := fmt.Sprintf("%s and %s", strings.Join(cycle[:len(cycle)-1], ", "), cycle[len(cycle)-1])
		return fmt.Errorf("manifest validation failed: cyclic dependency found between dependencies %s", depsInCycle)
	}
	return nil
}

func (md ManifestSection) toGraph() utils.Graph {
	g := utils.Graph{}
	for name, dep := range md {
		g[name] = dep.DependsOn
	}
	return g
}

func (md ManifestSection) IsEmpty() bool {
	return len(md) == 0
}
