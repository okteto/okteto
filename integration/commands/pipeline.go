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

// DeployPipelineOptions defines the options that can be added to a deploy command
type DeployPipelineOptions struct {
	Workdir     string
	Namespace   string
	Branch      string
	Repository  string
	OktetoHome  string
	Token       string
	Name        string
	Labels      []string
	ReuseParams bool
	Wait        bool
}

// DestroyPipelineOptions defines the options that can be added to a deploy command
type DestroyPipelineOptions struct {
	Workdir    string
	Namespace  string
	Name       string
	OktetoHome string
	Token      string
}

// RunOktetoDeployPipeline runs an okteto deploy command
func RunOktetoDeployPipeline(oktetoPath string, deployOptions *DeployPipelineOptions) error {
	cmd := exec.Command(oktetoPath, "pipeline", "deploy")
	if deployOptions.Workdir != "" {
		cmd.Dir = deployOptions.Workdir
	}

	if deployOptions.Branch != "" {
		cmd.Args = append(cmd.Args, "--branch", deployOptions.Branch)
	}
	if deployOptions.Repository != "" {
		cmd.Args = append(cmd.Args, "--repository", deployOptions.Repository)
	}
	if deployOptions.Namespace != "" {
		cmd.Args = append(cmd.Args, "--namespace", deployOptions.Namespace)
	}
	if deployOptions.Wait {
		cmd.Args = append(cmd.Args, "--wait")
	}

	if deployOptions.ReuseParams {
		cmd.Args = append(cmd.Args, "--reuse-params")
	}

	if deployOptions.Name != "" {
		cmd.Args = append(cmd.Args, "--name", deployOptions.Name)
	}

	for _, l := range deployOptions.Labels {
		cmd.Args = append(cmd.Args, "--label", l)
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
		log.Printf("%s: %s", cmd.String(), string(o))
		return fmt.Errorf("%s: %s", cmd.String(), string(o))
	}
	return nil
}

// RunOktetoPipelineDestroy runs an okteto destroy command
func RunOktetoPipelineDestroy(oktetoPath string, destroyOptions *DestroyPipelineOptions) error {
	log.Printf("okteto destroy %s", oktetoPath)
	cmd := exec.Command(oktetoPath, "pipeline", "destroy")
	if destroyOptions.Workdir != "" {
		cmd.Dir = destroyOptions.Workdir
	}
	if destroyOptions.Name != "" {
		cmd.Args = append(cmd.Args, "--name", destroyOptions.Name)
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
