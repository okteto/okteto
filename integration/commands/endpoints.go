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
)

// EndpointOptions defines the options that can be added to a deploy command
type EndpointOptions struct {
	Workdir    string
	Output     string
	Namespace  string
	OktetoHome string
}

// RunOktetoEndpoints runs an okteto deploy command
func RunOktetoEndpoints(oktetoPath string, endpointsOptions *EndpointOptions) ([]byte, error) {
	cmd := exec.Command(oktetoPath, "endpoints")
	cmd.Env = os.Environ()
	if endpointsOptions.Workdir != "" {
		cmd.Dir = endpointsOptions.Workdir
	}
	if endpointsOptions.Namespace != "" {
		cmd.Args = append(cmd.Args, "--namespace", endpointsOptions.Namespace)
	}
	if endpointsOptions.OktetoHome != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("OKTETO_HOME=%s", endpointsOptions.OktetoHome))
	}
	if endpointsOptions.Output != "" {
		cmd.Args = append(cmd.Args, "--output", endpointsOptions.Output)
	}
	log.Printf("Running '%s'", cmd.String())

	o, err := cmd.CombinedOutput()
	if err != nil {
		return o, fmt.Errorf("okteto deploy failed: %s - %w", string(o), err)
	}
	log.Printf("okteto deploy success")
	return o, nil
}
