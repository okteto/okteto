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

package commands

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/okteto/okteto/pkg/constants"
)

// GenerateSchemaOptions defines the options for the generate-schema command
type GenerateSchemaOptions struct {
	Workdir    string
	OutputFile string
	OktetoHome string
}

// GetOktetoGenerateSchemaCmdOutput runs an okteto generate-schema command and returns its output
func GetOktetoGenerateSchemaCmdOutput(oktetoPath string, generateSchemaOptions *GenerateSchemaOptions) ([]byte, error) {
	cmd := getGenerateSchemaCmd(oktetoPath, generateSchemaOptions)
	return cmd.CombinedOutput()
}

// RunOktetoGenerateSchema runs an okteto generate-schema command
func RunOktetoGenerateSchema(oktetoPath string, generateSchemaOptions *GenerateSchemaOptions) error {
	output, err := GetOktetoGenerateSchemaCmdOutput(oktetoPath, generateSchemaOptions)
	if err != nil {
		return fmt.Errorf("okteto generate-schema failed: %s - %w", string(output), err)
	}
	return nil
}

func getGenerateSchemaCmd(oktetoPath string, generateSchemaOptions *GenerateSchemaOptions) *exec.Cmd {
	cmd := exec.Command(oktetoPath, "generate-schema")
	if generateSchemaOptions.Workdir != "" {
		cmd.Dir = generateSchemaOptions.Workdir
	}
	if generateSchemaOptions.OutputFile != "" {
		cmd.Args = append(cmd.Args, "-o", generateSchemaOptions.OutputFile)
	}

	cmd.Env = os.Environ()
	if generateSchemaOptions.OktetoHome != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", constants.OktetoHomeEnvVar, generateSchemaOptions.OktetoHome))
	}

	return cmd
}
