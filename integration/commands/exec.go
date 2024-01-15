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

package commands

import (
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/model"
)

// ExecOptions are the options to add to an exec command
type ExecOptions struct {
	Namespace    string
	ManifestPath string
	Command      string
	OktetoHome   string
	Token        string
}

// RunExecCommand runs an exec command
func RunExecCommand(oktetoPath string, execOptions *ExecOptions) (string, error) {
	cmd := exec.Command(oktetoPath, "exec")
	cmd.Env = os.Environ()
	if execOptions.Namespace != "" {
		cmd.Args = append(cmd.Args, "-n", execOptions.Namespace)
	}

	if execOptions.ManifestPath != "" {
		cmd.Args = append(cmd.Args, "-f", execOptions.ManifestPath)
	}

	if execOptions.Command != "" {
		cmd.Args = append(cmd.Args, "--", execOptions.Command)
	}
	if v := os.Getenv(model.OktetoURLEnvVar); v != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", model.OktetoURLEnvVar, v))
	}

	if execOptions.OktetoHome != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", constants.OktetoHomeEnvVar, execOptions.OktetoHome))
	}
	if execOptions.Token != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", model.OktetoTokenEnvVar, execOptions.Token))
	}
	log.Printf("Running exec command: %s", cmd.String())
	bytes, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("okteto exec failed: %v - %s", err, string(bytes))
		return "", fmt.Errorf("okteto exec failed: %w - %s", err, string(bytes))
	}
	return string(bytes), nil
}
