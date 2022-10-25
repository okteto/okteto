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
	"os"
	"os/exec"
	"strings"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/model"
)

// RunOktetoKubeconfig runs okteto kubeconfig command
func RunOktetoKubeconfig(oktetoPath, oktetoHome string) error {
	args := []string{"kubeconfig"}
	cmd := exec.Command(oktetoPath, args...)
	cmd.Env = os.Environ()
	if v := os.Getenv(model.OktetoURLEnvVar); v != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", model.OktetoURLEnvVar, v))
	}

	if oktetoHome != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", constants.OktetoHomeEnvVar, oktetoHome))
	}
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", oktetoPath, strings.Join(args, " "), string(o))
	}
	fmt.Printf("kubeconfig output: %s", o)
	return nil
}
