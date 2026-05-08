// Copyright 2026 The Okteto Authors
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
	"strings"
)

const secretFormatHint = `

Secrets in your Okteto manifest must be valid file paths or environment variable names. Use one of these formats:

  # Short form (file path)
  secrets:
    my_secret: /path/to/secret/file

  # Explicit file key
  secrets:
    my_secret:
      file: /path/to/secret/file

  # Environment variable as secret value
  secrets:
    my_secret:
      env: MY_ENV_VAR`

// Secret represents a single build secret — either a file path or an env var name.
type Secret struct {
	File string `yaml:"file,omitempty"`
	Env  string `yaml:"env,omitempty"`
}

// String returns a canonical representation used for hashing (e.g. "file:/path" or "env:VAR").
func (s Secret) String() string {
	if s.Env != "" {
		return fmt.Sprintf("env:%s", s.Env)
	}
	return fmt.Sprintf("file:%s", s.File)
}

// UnmarshalYAML handles both short (string) and long (map) forms:
//
//	my_secret: /path/to/file        → Secret{File: "/path/to/file"}
//	my_secret:
//	  file: /path/to/file           → Secret{File: "/path/to/file"}
//	my_secret:
//	  env: MY_ENV_VAR               → Secret{Env: "MY_ENV_VAR"}
func (s *Secret) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var shorthand string
	if err := unmarshal(&shorthand); err == nil {
		s.File = shorthand
		return nil
	}

	// Decode into a plain map so that unknown keys produce a clean, user-facing
	// error rather than leaking internal type names (e.g. "build.rawSecret").
	var raw map[string]string
	if err := unmarshal(&raw); err != nil {
		return fmt.Errorf("%w%s", err, secretFormatHint)
	}

	for k := range raw {
		if k != "file" && k != "env" {
			return fmt.Errorf("unknown secret key %q — only 'file' and 'env' are allowed%s", k, secretFormatHint)
		}
	}

	file, envName := strings.TrimSpace(raw["file"]), strings.TrimSpace(raw["env"])
	if file != "" && envName != "" {
		return fmt.Errorf("secret cannot specify both 'file' and 'env'%s", secretFormatHint)
	}
	if file == "" && envName == "" {
		return fmt.Errorf("secret must specify either 'file' or 'env'%s", secretFormatHint)
	}
	s.File = file
	s.Env = envName
	return nil
}

// Secrets represents the secrets to be injected to the build of the image
type Secrets map[string]Secret

// UnmarshalYAML deserializes the secrets map.
func (s *Secrets) UnmarshalYAML(unmarshal func(interface{}) error) error {
	raw := map[string]Secret{}
	if err := unmarshal(&raw); err != nil {
		return err
	}
	*s = raw
	return nil
}
