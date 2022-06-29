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
)

// KubectlOptions defines the options that can be added to a build command
type KubectlOptions struct {
	Workdir    string
	Namespace  string
	File       string
	ConfigFile string
}

// RunKubectlApply runs kubectl apply command
func RunKubectlApply(kubectlBinary string, kubectlOpts *KubectlOptions) error {
	args := []string{"apply", "-n", kubectlOpts.Namespace, "-f", kubectlOpts.File}
	cmd := exec.Command(kubectlBinary, args...)
	if kubectlOpts.ConfigFile != "" {
		cmd.Args = append(cmd.Args, "--kubeconfig", kubectlOpts.ConfigFile)
	}
	if kubectlOpts.Workdir != "" {
		cmd.Dir = kubectlOpts.Workdir
	}

	cmd.Env = os.Environ()

	if o, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("kubectl apply failed: %s", string(o))
	}
	return nil
}
