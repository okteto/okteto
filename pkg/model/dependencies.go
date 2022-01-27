package model

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
	Repository   string      `json:"repository,omitempty" yaml:"repository,omitempty"`
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

	dependency.Repository = rawDependency.Repository
	dependency.ManifestPath = rawDependency.ManifestPath
	dependency.Branch = rawDependency.Branch
	dependency.Variables = rawDependency.Variables

	return nil
}
