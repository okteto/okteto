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
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/model"
)

// KubeconfigOpts represents the options for the kubeconfig command
type KubeconfigOpts struct {
	OktetoHome     string
	Token          string
	UseStaticToken bool
}

// RunOktetoKubeconfig runs okteto kubeconfig command
func RunOktetoKubeconfig(oktetoPath string, opts *KubeconfigOpts) error {
	args := []string{"kubeconfig"}
	cmd := exec.Command(oktetoPath, args...)
	cmd.Env = os.Environ()
	if v := os.Getenv(model.OktetoURLEnvVar); v != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", model.OktetoURLEnvVar, v))
	}

	if opts.OktetoHome != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", constants.OktetoHomeEnvVar, opts.OktetoHome))
	}
	if opts.Token != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", model.OktetoTokenEnvVar, opts.Token))
	}

	if opts.UseStaticToken {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", context.OktetoUseStaticKubetokenEnvVar, strconv.FormatBool(opts.UseStaticToken)))
	}
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", oktetoPath, strings.Join(args, " "), string(o))
	}
	fmt.Printf("kubeconfig output: %s", o)
	return nil
}
