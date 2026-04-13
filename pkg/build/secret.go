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
	"os"
	"path/filepath"
	"strings"

	"github.com/okteto/okteto/pkg/env"
)

// Secret represents a single build secret — either a file path or an env var name.
type Secret struct {
	File string `yaml:"file,omitempty"`
	Env  string `yaml:"env,omitempty"`
}

// UnmarshalYAML handles both short (string) and long (struct) forms:
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

	type rawSecret Secret // avoid infinite recursion
	var raw rawSecret
	if err := unmarshal(&raw); err != nil {
		return err
	}
	if raw.File != "" && raw.Env != "" {
		return fmt.Errorf("secret cannot specify both 'file' and 'env'")
	}
	if raw.File == "" && raw.Env == "" {
		return fmt.Errorf("secret must specify either 'file' or 'env'")
	}
	s.File = raw.File
	s.Env = raw.Env
	return nil
}

// Secrets represents the secrets to be injected to the build of the image
type Secrets map[string]Secret

func (i *Info) expandSecrets() error {
	for k, s := range i.Secrets {
		if s.File == "" {
			// env-based secrets don't need path expansion
			continue
		}
		val := s.File
		if strings.HasPrefix(val, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			val = filepath.Join(home, val[2:])
		}
		expanded, err := env.ExpandEnv(val)
		if err != nil {
			return err
		}
		s.File = expanded
		i.Secrets[k] = s
	}
	return nil
}
