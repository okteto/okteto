// Copyright 2024 The Okteto Authors
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

package dotenv

import (
	"fmt"

	"github.com/compose-spec/godotenv"
	"github.com/okteto/okteto/pkg/filesystem"
	"github.com/okteto/okteto/pkg/vars"
	"github.com/spf13/afero"
)

const DefaultDotEnvFile = ".env"

// Load loads the content of a .env file into the varManager
func Load(filepath string, varManager *vars.Manager, fs afero.Fs) error {
	if !filesystem.FileExistsWithFilesystem(filepath, fs) {
		return nil
	}

	content, err := afero.ReadFile(fs, filepath)
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	expanded, err := varManager.Expand(string(content))
	if err != nil {
		return fmt.Errorf("error expanding dot env file: %w", err)
	}
	expandedVars, err := godotenv.UnmarshalBytes([]byte(expanded))
	if err != nil {
		return fmt.Errorf("error parsing dot env file: %w", err)
	}

	for k, v := range expandedVars {
		varManager.AddDotEnvVar(k, v)
	}

	return nil
}
