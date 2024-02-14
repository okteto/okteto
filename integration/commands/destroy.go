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
	"log"
	"os"
	"os/exec"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/model"
)

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
		return string(o), fmt.Errorf("okteto destroy failed: %s - %w", string(o), err)
	}
	log.Printf("okteto destroy success")
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
