package build

// DependsOn represents the images that needs to be built before
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
