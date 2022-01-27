package model

import (
	"errors"
	"fmt"
	"strings"
)

type dependenciesRaw struct {
	Repository   string      `json:"repository,omitempty" yaml:"repository,omitempty"`
	ManifestPath string      `json:"manifest,omitempty" yaml:"manifest,omitempty"`
	Branch       string      `json:"branch,omitempty" yaml:"branch,omitempty"`
	Variables    Environment `json:"variables,omitempty" yaml:"variables,omitempty"`
}

// ManifestDependencies represents the map of dependencies at a manifest
type ManifestDependencies map[string]*Dependency

// Dependency represents a dependency object at the manifest
type Dependency struct {
	Repository   string      `json:"repository" yaml:"repository"`
	ManifestPath string      `json:"manifest,omitempty" yaml:"manifest,omitempty"`
	Branch       string      `json:"branch,omitempty" yaml:"branch,omitempty"`
	Variables    Environment `json:"variables,omitempty" yaml:"variables,omitempty"`
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (dependency *Dependency) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rawString string
	err := unmarshal(&rawString)
	if err == nil {
		dependency.Repository = rawString
		return nil
	}

	var rawDependency dependenciesRaw
	err = unmarshal(&rawDependency)
	if err != nil {
		return err
	}

	if rawDependency.Repository == "" {
		return errors.New("repository is mandatory")
	}
	dependency.Repository = rawDependency.Repository
	dependency.ManifestPath = rawDependency.ManifestPath
	dependency.Branch = rawDependency.Branch
	dependency.Variables = rawDependency.Variables

	return nil
}

// TransformToPipelineCommand returns the command to deploy the pipeline
func (dependency *Dependency) TransformToPipelineCommand() DeployCommand {
	comm := []string{"okteto pipeline deploy"}

	repo := fmt.Sprintf("-r %s", dependency.Repository)
	comm = append(comm, repo)

	var branch, file, variables string

	if dependency.Branch != "" {
		branch = fmt.Sprintf("-b %s", dependency.Branch)
		comm = append(comm, branch)
	}

	if dependency.ManifestPath != "" {
		file = fmt.Sprintf("-f %s", dependency.ManifestPath)
		comm = append(comm, file)
	}

	if len(dependency.Variables) > 0 {
		vars := SerializeBuildArgs(dependency.Variables)
		variables = fmt.Sprintf("-v %s", strings.Join(vars, ","))
		comm = append(comm, variables)
	}

	return DeployCommand{Name: strings.Join(comm, " "), Command: strings.Join(comm, " ")}
}
