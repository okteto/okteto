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
	"path/filepath"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/model"
)

// TestOptions defines the options that can be added to a test command
type TestOptions struct {
	TestName     string
	Workdir      string
	ManifestPath string
	LogLevel     string
	LogOutput    string
	Namespace    string
	OktetoHome   string
	Token        string
	NoCache      bool
}

// RunOktetoTestAndGetOutput runs an okteto deploy command and returns the output
func RunOktetoTestAndGetOutput(oktetoPath string, testOptions *TestOptions) (string, error) {
	cmd := getTestCmd(oktetoPath, testOptions)
	log.Printf("Running '%s'", cmd.String())
	o, err := cmd.CombinedOutput()
	if err != nil {
		return string(o), fmt.Errorf("okteto test failed: %s - %w", string(o), err)
	}
	log.Printf("okteto test success")
	return string(o), nil
}

func getTestCmd(oktetoPath string, testOptions *TestOptions) *exec.Cmd {
	cmd := exec.Command(oktetoPath, "test")
	if testOptions.Workdir != "" {
		cmd.Dir = testOptions.Workdir
	}

	cmd.Args = append(cmd.Args, testOptions.TestName)
	if testOptions.NoCache {
		cmd.Args = append(cmd.Args, "--no-cache")
	}
	cmd.Env = os.Environ()
	if v := os.Getenv(model.OktetoURLEnvVar); v != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", model.OktetoURLEnvVar, v))
	}
	if testOptions.ManifestPath != "" {
		cmd.Args = append(cmd.Args, "-f", testOptions.ManifestPath)
	}
	if testOptions.LogLevel != "" {
		cmd.Args = append(cmd.Args, "--log-level", testOptions.LogLevel)
	}
	if testOptions.Namespace != "" {
		cmd.Args = append(cmd.Args, "--namespace", testOptions.Namespace)
	}
	if testOptions.LogOutput != "" {
		cmd.Args = append(cmd.Args, "--log-output", testOptions.LogOutput)
	}
	if testOptions.OktetoHome != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", constants.OktetoHomeEnvVar, testOptions.OktetoHome))
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", constants.KubeConfigEnvVar, filepath.Join(testOptions.OktetoHome, ".kube", "config")))
	}
	if testOptions.Token != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", model.OktetoTokenEnvVar, testOptions.Token))
	}

	return cmd
}
