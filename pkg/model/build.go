package model

type rawBuildParams struct {
	Image     string   `json:"image,omitempty" yaml:"image,omitempty"`
	Context   string   `json:"context,omitempty" yaml:"context,omitempty"`
	File      string   `json:"dockerfile,omitempty" yaml:"dockerfile,omitempty"`
	Target    string   `json:"target,omitempty" yaml:"target,omitempty"`
	Args      []string `json:"args,omitempty" yaml:"args,omitempty"`
	Secrets   []string `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	CacheFrom []string `json:"cacheFrom,omitempty" yaml:"cacheFrom,omitempty"`
}

type BuildParams struct {
	Image     string   `json:"image,omitempty" yaml:"image,omitempty"`
	Context   string   `json:"context,omitempty" yaml:"context,omitempty"`
	File      string   `json:"dockerfile,omitempty" yaml:"dockerfile,omitempty"`
	Target    string   `json:"target,omitempty" yaml:"target,omitempty"`
	Args      []string `json:"args,omitempty" yaml:"args,omitempty"`
	Secrets   []string `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	CacheFrom []string `json:"cacheFrom,omitempty" yaml:"cacheFrom,omitempty"`
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (build *BuildParams) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rawString string
	err := unmarshal(&rawString)
	if err == nil {
		build.Context = rawString

		return nil
	} else {
		var buildParams rawBuildParams
		err = unmarshal(&buildParams)
		if err != nil {
			return err
		}

		build.Image = buildParams.Image
		build.Context = buildParams.Context
		build.File = buildParams.File
		build.Target = buildParams.Target
		build.Args = buildParams.Args
		return nil
	}

}
