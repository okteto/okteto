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

// RunOktetoPush runs an okteto push command
func RunOktetoPush(oktetoPath, workdir string) error {
	cmd := exec.Command(oktetoPath, "push")
	if workdir != "" {
		cmd.Dir = workdir
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", constants.OktetoHomeEnvVar, workdir))
	}
	if v := os.Getenv(model.OktetoURLEnvVar); v != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", model.OktetoURLEnvVar, v))
	}

	log.Printf("Running '%s'", cmd.String())

	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("okteto stack deploy failed: %s - %w", string(o), err)
	}
	log.Printf("okteto stack deploy success")
	return nil
}
