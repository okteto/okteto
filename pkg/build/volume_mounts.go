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
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

type VolumeMounts struct {
	LocalPath  string `yaml:"local_path,omitempty"`
	RemotePath string `yaml:"remote_path,omitempty"`
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
