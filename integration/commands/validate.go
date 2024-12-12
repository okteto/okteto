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

// ValidateOptions defines the options for the validate command
type ValidateOptions struct {
	Workdir      string
	ManifestPath string
	OktetoHome   string
}

// GetOktetoValidateCmdOutput runs an okteto validate command and returns its output
func GetOktetoValidateCmdOutput(oktetoPath string, validateOptions *ValidateOptions) ([]byte, error) {
	cmd := getValidateCmd(oktetoPath, validateOptions)
	return cmd.CombinedOutput()
}

// RunOktetoValidate runs an okteto validate command
func RunOktetoValidate(oktetoPath string, validateOptions *ValidateOptions) error {
	output, err := GetOktetoValidateCmdOutput(oktetoPath, validateOptions)
	if err != nil {
		return fmt.Errorf("okteto validate failed: %s - %w", string(output), err)
	}
	return nil
}

func getValidateCmd(oktetoPath string, validateOptions *ValidateOptions) *exec.Cmd {
	cmd := exec.Command(oktetoPath, "validate")
	if validateOptions.Workdir != "" {
		cmd.Dir = validateOptions.Workdir
	}
	if validateOptions.ManifestPath != "" {
		cmd.Args = append(cmd.Args, "-f", validateOptions.ManifestPath)
	}

	cmd.Env = os.Environ()
	if validateOptions.OktetoHome != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", constants.OktetoHomeEnvVar, validateOptions.OktetoHome))
	}

	return cmd
}
