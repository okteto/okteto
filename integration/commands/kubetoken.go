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
	"bytes"
	"fmt"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/model"
	"log"
	"os"
	"os/exec"
)

// RunOktetoKubetoken runs okteto kubetoken command
func RunOktetoKubetoken(oktetoPath, oktetoHome string) (bytes.Buffer, error) {
	args := []string{"kubetoken"}
	cmd := exec.Command(oktetoPath, args...)

	cmd.Env = os.Environ()
	if v := os.Getenv(model.OktetoURLEnvVar); v != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", model.OktetoURLEnvVar, v))
	}
	if oktetoHome != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", constants.OktetoHomeEnvVar, oktetoHome))
	}

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	log.Printf("Running up command: %s", cmd.String())

	err := cmd.Run()
	if err != nil {
		log.Printf("okteto kubetoken failed: %v", err)
		log.Printf("okteto kubetoken output err: \n%s", out.String())
		return out, err
	}

	return out, nil
}
