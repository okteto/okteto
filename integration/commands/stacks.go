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

// StackDeployOptions defines the options that can be added to a deploy command
type StackDeployOptions struct {
	Workdir      string
	ManifestPath string
	OktetoHome   string
	Token        string
	Build        bool
}

// StackDestroyOptions defines the options that can be added to a deploy command
type StackDestroyOptions struct {
	Workdir      string
	ManifestPath string
	OktetoHome   string
	Token        string
}

// RunOktetoStackDeploy runs an okteto deploy command
func RunOktetoStackDeploy(oktetoPath string, deployOptions *StackDeployOptions) error {
	cmd := exec.Command(oktetoPath, "stack", "deploy")
	if deployOptions.Workdir != "" {
		cmd.Dir = deployOptions.Workdir
	}
	if deployOptions.Build {
		cmd.Args = append(cmd.Args, "--build")
	}
	if deployOptions.ManifestPath != "" {
		cmd.Args = append(cmd.Args, "-f", deployOptions.ManifestPath)
	}

	if v := os.Getenv(model.OktetoURLEnvVar); v != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", model.OktetoURLEnvVar, v))
	}

	if deployOptions.OktetoHome != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", constants.OktetoHomeEnvVar, deployOptions.OktetoHome))
	}
	if deployOptions.Token != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", model.OktetoTokenEnvVar, deployOptions.Token))
	}

	log.Printf("Running '%s'", cmd.String())

	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("okteto stack deploy failed: %s - %w", string(o), err)
	}
	log.Printf("okteto stack deploy success")
	return nil
}

// RunOktetoStackDestroy runs an okteto deploy command
func RunOktetoStackDestroy(oktetoPath string, deployOptions *StackDestroyOptions) error {
	cmd := exec.Command(oktetoPath, "stack", "destroy")
	if deployOptions.Workdir != "" {
		cmd.Dir = deployOptions.Workdir
	}
	if deployOptions.ManifestPath != "" {
		cmd.Args = append(cmd.Args, "-f", deployOptions.ManifestPath)
	}

	if v := os.Getenv(model.OktetoURLEnvVar); v != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", model.OktetoURLEnvVar, v))
	}

	if deployOptions.OktetoHome != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", constants.OktetoHomeEnvVar, deployOptions.OktetoHome))
	}
	if deployOptions.Token != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", model.OktetoTokenEnvVar, deployOptions.Token))
	}
	log.Printf("Running '%s'", cmd.String())

	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("okteto stack destroy failed: %s - %w", string(o), err)
	}
	log.Printf("okteto stack destroy success")
	return nil
}
