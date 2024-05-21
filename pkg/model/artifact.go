package model

type Artifact struct {
	Path        string `yaml:"path,omitempty"`
	Destination string `yaml:"destination,omitempty"`
}

func (t *Artifact) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var path string
	err := unmarshal(&path)
	if err == nil {
		t.Path = path
		t.Destination = path
		return nil
	}

	// prevent recursion
	type artifactAlias Artifact
	var extendedArtifact artifactAlias
	err = unmarshal(&extendedArtifact)
	if err != nil {
		return err
	}
	if extendedArtifact.Destination == "" {
		extendedArtifact.Destination = extendedArtifact.Path
	}
	*t = Artifact(extendedArtifact)
	return nil
}
