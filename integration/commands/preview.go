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

// DeployPreviewOptions defines the options that can be added to a deploy command
type DeployPreviewOptions struct {
	Workdir    string
	Namespace  string
	Scope      string
	Branch     string
	Repository string
	OktetoHome string
	Token      string
	Wait       bool
}

// DestroyPreviewOptions defines the options that can be added to a deploy command
type DestroyPreviewOptions struct {
	Workdir    string
	Namespace  string
	OktetoHome string
	Token      string
}

// RunOktetoDeployPreview runs an okteto deploy command
func RunOktetoDeployPreview(oktetoPath string, deployOptions *DeployPreviewOptions) error {
	cmd := exec.Command(oktetoPath, "preview", "deploy", deployOptions.Namespace)
	if deployOptions.Workdir != "" {
		cmd.Dir = deployOptions.Workdir
	}
	if deployOptions.Scope != "" {
		cmd.Args = append(cmd.Args, "--scope", deployOptions.Scope)
	} else {
		cmd.Args = append(cmd.Args, "--scope", "personal")
	}
	if deployOptions.Branch != "" {
		cmd.Args = append(cmd.Args, "--branch", deployOptions.Branch)
	} else {
		cmd.Args = append(cmd.Args, "--branch", "master")
	}
	if deployOptions.Repository != "" {
		cmd.Args = append(cmd.Args, "--repository", deployOptions.Repository)
	}
	if deployOptions.Wait {
		cmd.Args = append(cmd.Args, "--wait")
	}

	cmd.Env = os.Environ()
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
		return fmt.Errorf("%s: %s", cmd.String(), string(o))
	}
	return nil
}

// RunOktetoPreviewDestroy runs an okteto destroy command
func RunOktetoPreviewDestroy(oktetoPath string, destroyOptions *DestroyPreviewOptions) error {
	log.Printf("okteto destroy %s", oktetoPath)
	cmd := exec.Command(oktetoPath, "preview", "destroy", destroyOptions.Namespace)
	if destroyOptions.Workdir != "" {
		cmd.Dir = destroyOptions.Workdir
	}
	if v := os.Getenv(model.OktetoURLEnvVar); v != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", model.OktetoURLEnvVar, v))
	}

	if destroyOptions.OktetoHome != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", constants.OktetoHomeEnvVar, destroyOptions.OktetoHome))
	}
	if destroyOptions.Token != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", model.OktetoTokenEnvVar, destroyOptions.Token))
	}
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("okteto deploy failed: %s - %w", string(o), err)
	}
	log.Printf("okteto destroy success")
	return nil
}
