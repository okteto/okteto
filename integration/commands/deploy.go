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
	"path/filepath"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/model"
)

// DeployOptions defines the options that can be added to a deploy command
type DeployOptions struct {
	Workdir          string
	ManifestPath     string
	LogLevel         string
	LogOutput        string
	Namespace        string
	OktetoHome       string
	Token            string
	Name             string
	Variables        []string
	ServicesToDeploy []string
	Build            bool
	IsRemote         bool
}

// DestroyOptions defines the options that can be added to a deploy command
type DestroyOptions struct {
	Workdir      string
	ManifestPath string
	Namespace    string
	OktetoHome   string
	Token        string
	Name         string
	IsRemote     bool
}

// GetOktetoDeployCmdOutput runs an okteto deploy command
func GetOktetoDeployCmdOutput(oktetoPath string, deployOptions *DeployOptions) ([]byte, error) {
	cmd := getDeployCmd(oktetoPath, deployOptions)
	log.Printf("Running '%s'", cmd.String())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, err
	}
	log.Printf("okteto deploy success")
	return output, nil
}

// RunOktetoDeploy runs an okteto deploy command
func RunOktetoDeploy(oktetoPath string, deployOptions *DeployOptions) error {
	output, err := GetOktetoDeployCmdOutput(oktetoPath, deployOptions)
	if err != nil {
		return fmt.Errorf("okteto deploy failed: %s - %w", string(output), err)
	}
	return err
}

// RunOktetoDeployAndGetOutput runs an okteto deploy command and returns the output
func RunOktetoDeployAndGetOutput(oktetoPath string, deployOptions *DeployOptions) (string, error) {
	cmd := getDeployCmd(oktetoPath, deployOptions)
	log.Printf("Running '%s'", cmd.String())
	o, err := cmd.CombinedOutput()
	if err != nil {
		return string(o), fmt.Errorf("okteto deploy failed: %s - %w", string(o), err)
	}
	log.Printf("okteto deploy success")
	return string(o), nil
}

// RunOktetoDestroy runs an okteto destroy command
func RunOktetoDestroy(oktetoPath string, destroyOptions *DestroyOptions) error {
	log.Printf("okteto destroy %s", oktetoPath)
	cmd := getDestroyCmd(oktetoPath, destroyOptions)
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("okteto deploy failed: %s - %w", string(o), err)
	}
	log.Printf("okteto destroy success")
	return nil
}

// RunOktetoDestroyAndGetOutput runs an okteto destroy command and returns the output
func RunOktetoDestroyAndGetOutput(oktetoPath string, destroyOptions *DestroyOptions) (string, error) {
	cmd := getDestroyCmd(oktetoPath, destroyOptions)
	log.Printf("Running '%s'", cmd.String())
	o, err := cmd.CombinedOutput()
	if err != nil {
		return string(o), fmt.Errorf("okteto deploy failed: %s - %w", string(o), err)
	}
	log.Printf("okteto deploy success")
	return string(o), nil
}

// RunOktetoDestroyRemote runs an okteto destroy command in remote
func RunOktetoDestroyRemote(oktetoPath string, destroyOptions *DestroyOptions) error {
	log.Printf("okteto destroy %s", oktetoPath)
	cmd := getDestroyCmd(oktetoPath, destroyOptions)
	cmd.Args = append(cmd.Args, "--remote")

	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("okteto deploy --remote failed: %s - %w", string(o), err)
	}
	log.Printf("okteto destroy success")
	return nil
}

func getDestroyCmd(oktetoPath string, destroyOptions *DestroyOptions) *exec.Cmd {
	cmd := exec.Command(oktetoPath, "destroy")
	if destroyOptions.Workdir != "" {
		cmd.Dir = destroyOptions.Workdir
	}
	if destroyOptions.Name != "" {
		cmd.Args = append(cmd.Args, "--name", destroyOptions.Name)
	}
	if destroyOptions.ManifestPath != "" {
		cmd.Args = append(cmd.Args, "-f", destroyOptions.ManifestPath)
	}
	if destroyOptions.Namespace != "" {
		cmd.Args = append(cmd.Args, "--namespace", destroyOptions.Namespace)
	}
	if destroyOptions.IsRemote {
		cmd.Args = append(cmd.Args, "--remote")
	}
	cmd.Env = os.Environ()
	if v := os.Getenv(model.OktetoURLEnvVar); v != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", model.OktetoURLEnvVar, v))
	}

	if destroyOptions.OktetoHome != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", constants.OktetoHomeEnvVar, destroyOptions.OktetoHome))
	}
	if destroyOptions.Token != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", model.OktetoTokenEnvVar, destroyOptions.Token))
	}
	return cmd
}

func getDeployCmd(oktetoPath string, deployOptions *DeployOptions) *exec.Cmd {
	cmd := exec.Command(oktetoPath, "deploy")
	if deployOptions.Workdir != "" {
		cmd.Dir = deployOptions.Workdir
	}
	if len(deployOptions.ServicesToDeploy) > 0 {
		cmd.Args = append(cmd.Args, deployOptions.ServicesToDeploy...)
	}
	if deployOptions.ManifestPath != "" {
		cmd.Args = append(cmd.Args, "-f", deployOptions.ManifestPath)
	}
	if deployOptions.Build {
		cmd.Args = append(cmd.Args, "--build")
	}
	if deployOptions.LogLevel != "" {
		cmd.Args = append(cmd.Args, "--log-level", deployOptions.LogLevel)
	}
	if deployOptions.Namespace != "" {
		cmd.Args = append(cmd.Args, "--namespace", deployOptions.Namespace)
	}
	if deployOptions.LogOutput != "" {
		cmd.Args = append(cmd.Args, "--log-output", deployOptions.LogOutput)
	}
	if deployOptions.Name != "" {
		cmd.Args = append(cmd.Args, "--name", deployOptions.Name)
	}
	for _, v := range deployOptions.Variables {
		cmd.Args = append(cmd.Args, "--var", v)
	}
	if deployOptions.IsRemote {
		cmd.Args = append(cmd.Args, "--remote")
	}
	cmd.Env = os.Environ()
	if v := os.Getenv(model.OktetoURLEnvVar); v != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", model.OktetoURLEnvVar, v))
	}

	if deployOptions.OktetoHome != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", constants.OktetoHomeEnvVar, deployOptions.OktetoHome))
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", constants.KubeConfigEnvVar, filepath.Join(deployOptions.OktetoHome, ".kube", "config")))
	}
	if deployOptions.Token != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", model.OktetoTokenEnvVar, deployOptions.Token))
	}

	return cmd
}
