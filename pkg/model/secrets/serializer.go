// Copyright 2021 The Okteto Authors
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

package secrets

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/okteto/okteto/pkg/model/environment"
)

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (s *Secret) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw string
	err := unmarshal(&raw)
	if err != nil {
		return err
	}

	rawExpanded, err := environment.ExpandEnv(raw)
	if err != nil {
		return err
	}
	parts := strings.Split(rawExpanded, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return fmt.Errorf("secrets must follow the syntax 'LOCAL_PATH:REMOTE_PATH:MODE'")
	}
	s.LocalPath = parts[0]
	if err := checkFileAndNotDirectory(s.LocalPath); err != nil {
		return err
	}
	s.RemotePath = parts[1]
	if !strings.HasPrefix(s.RemotePath, "/") {
		return fmt.Errorf("Secret remote path '%s' must be an absolute path", s.RemotePath)
	}
	if len(parts) == 3 {
		mode, err := strconv.ParseInt(parts[2], 8, 32)
		if err != nil {
			return fmt.Errorf("error parsing secret '%s' mode: %s", parts[0], err)
		}
		s.Mode = int32(mode)
	} else {
		s.Mode = 420
	}
	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (s Secret) MarshalYAML() (interface{}, error) {
	if s.Mode == 420 {
		return fmt.Sprintf("%s:%s:%s", s.LocalPath, s.RemotePath, strconv.FormatInt(int64(s.Mode), 8)), nil
	}
	return fmt.Sprintf("%s:%s", s.LocalPath, s.RemotePath), nil
}

func checkFileAndNotDirectory(path string) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("File '%s' not found. Please make sure the file exists", path)
	}
	if fileInfo.Mode().IsRegular() {
		return nil
	}
	return fmt.Errorf("Secret '%s' is not a regular file", path)
}
