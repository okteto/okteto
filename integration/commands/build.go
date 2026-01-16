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
	"io"
	"os"
	"os/exec"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/model"
)

// BuildOptions defines the options that can be added to a build command
type BuildOptions struct {
	Workdir      string
	ManifestPath string
	Tag          string
	Namespace    string
	OktetoHome   string
	Token        string
	SvcsToBuild  []string
	NoCache      bool
}

// RunOktetoBuild runs an okteto build command
func RunOktetoBuild(oktetoPath string, buildOptions *BuildOptions) error {
	cmd := GetOktetoBuildCmd(oktetoPath, buildOptions)
	return ExecOktetoBuildCmd(cmd)
}

// GetOktetoBuildCmd returns an exec.Cmd with the needed values given buildOpts
func GetOktetoBuildCmd(oktetoPath string, buildOptions *BuildOptions) *exec.Cmd {
	cmd := exec.Command(oktetoPath)
	cmd.Args = append(cmd.Args, "build")
	if buildOptions.ManifestPath != "" {
		cmd.Args = append(cmd.Args, "-f", buildOptions.ManifestPath)
	}
	if buildOptions.Tag != "" {
		cmd.Args = append(cmd.Args, "-t", buildOptions.Tag)
	}
	if buildOptions.Workdir != "" {
		cmd.Dir = buildOptions.Workdir
	}
	if buildOptions.Namespace != "" {
		cmd.Args = append(cmd.Args, "--namespace", buildOptions.Namespace)
	}
	if len(buildOptions.SvcsToBuild) > 0 {
		cmd.Args = append(cmd.Args, buildOptions.SvcsToBuild...)
	}

	if v := os.Getenv(model.OktetoURLEnvVar); v != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", model.OktetoURLEnvVar, v))
	}

	if buildOptions.OktetoHome != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", constants.OktetoHomeEnvVar, buildOptions.OktetoHome))
	}
	if buildOptions.Token != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", model.OktetoTokenEnvVar, buildOptions.Token))
	}

	if buildOptions.NoCache {
		cmd.Args = append(cmd.Args, "--no-cache")
	}

	return cmd
}

// ExecOktetoBuildCmd runs an okteto build command
func ExecOktetoBuildCmd(cmd *exec.Cmd) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start okteto build: %w", err)
	}

	go func() {
		if _, err := io.Copy(os.Stdout, stdout); err != nil {
			fmt.Fprintf(os.Stderr, "error copying stdout: %v\n", err)
		}
	}()

	go func() {
		if _, err := io.Copy(os.Stderr, stderr); err != nil {
			fmt.Fprintf(os.Stderr, "error copying stderr: %v\n", err)
		}
	}()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("okteto build failed: \nerror: %w", err)
	}
	return nil
}
