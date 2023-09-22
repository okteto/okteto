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

	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/model"
)

// ContextOptions defines the options to create a context command
type ContextOptions struct {
	Workdir       string
	OktetoHome    string
	Namespace     string
	Token         string
	SkipTlsVerify bool
}

// RunOktetoContext creates and runs an okteto context command
func RunOktetoContext(oktetoPath string, contextOptions *ContextOptions) error {
	cmd := exec.Command(oktetoPath)
	cmd.Args = append(cmd.Args, "context")
	if contextOptions.Workdir != "" {
		cmd.Dir = contextOptions.Workdir
	}
	if contextOptions.Namespace != "" {
		cmd.Args = append(cmd.Args, "--namespace", contextOptions.Namespace)
	}
	if contextOptions.SkipTlsVerify {
		cmd.Args = append(cmd.Args, "--insecure-skip-tls-verify")
	}

	if v := os.Getenv(model.OktetoURLEnvVar); v != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", model.OktetoURLEnvVar, v))
	}

	if contextOptions.OktetoHome != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", constants.OktetoHomeEnvVar, contextOptions.OktetoHome))
	}
	if contextOptions.Token != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", model.OktetoTokenEnvVar, contextOptions.Token))
	}

	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("okteto context failed: \nerror: %s \noutput:\n %s", err.Error(), string(o))
	}
	return nil
}
