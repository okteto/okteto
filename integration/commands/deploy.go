// Copyright 2022 The Okteto Authors
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
	"os/exec"
)

// DeployOptions defines the options that can be added to a deploy command
type DeployOptions struct {
	Workdir      string
	ManifestPath string
}

// DestroyOptions defines the options that can be added to a deploy command
type DestroyOptions struct {
	Workdir      string
	ManifestPath string
}

// RunOktetoDeploy runs an okteto deploy command
func RunOktetoDeploy(oktetoPath string, deployOptions *DeployOptions) error {
	log.Printf("okteto deploy %s", oktetoPath)
	cmd := exec.Command(oktetoPath, "deploy")
	if deployOptions.Workdir != "" {
		cmd.Dir = deployOptions.Workdir
	}
	if deployOptions.ManifestPath != "" {
		cmd.Args = append(cmd.Args, "-f", deployOptions.ManifestPath)
	}

	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("okteto deploy failed: %s - %s", string(o), err)
	}
	log.Printf("okteto deploy success")
	return nil
}

// RunOktetoDestroy runs an okteto destroy command
func RunOktetoDestroy(oktetoPath string, destroyOptions *DestroyOptions) error {
	log.Printf("okteto destroy %s", oktetoPath)
	cmd := exec.Command(oktetoPath, "deploy")
	if destroyOptions.Workdir != "" {
		cmd.Dir = destroyOptions.Workdir
	}
	if destroyOptions.ManifestPath != "" {
		cmd.Args = append(cmd.Args, "-f", destroyOptions.ManifestPath)
	}

	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("okteto deploy failed: %s - %s", string(o), err)
	}
	log.Printf("okteto destroy success")
	return nil
}
