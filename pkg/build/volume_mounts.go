package build

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

type VolumeMounts struct {
	LocalPath  string
	RemotePath string
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
