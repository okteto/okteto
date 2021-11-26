// Copyright 2021 The Okteto Authors
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

package utils

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
)

type ManifestExecutor interface {
	Execute(command string, env []string) error
}

type Executor struct{}

// NewExecutor returns a new executor
func NewExecutor() *Executor {
	return &Executor{}
}

// InferApplicationName infers the application name from the folder received as parameter
func InferApplicationName(cwd string) string {
	repo, err := model.GetRepositoryURL(cwd)
	if err != nil {
		log.Info("inferring name from folder")
		return filepath.Base(cwd)
	}

	log.Info("inferring name from git repository URL")
	return model.TranslateURLToName(repo)
}

// Execute executes the specified command adding `env` to the execution environment
func (*Executor) Execute(command string, env []string) error {
	fmt.Printf("Running '%s'...\n", command)

	cmd := exec.Command("bash", "-c", command)
	cmd.Env = append(os.Environ(), env...)

	r, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	cmd.Stderr = cmd.Stdout
	done := make(chan struct{})
	scanner := bufio.NewScanner(r)

	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println(line)
		}
		done <- struct{}{}
	}()

	if err := cmd.Start(); err != nil {
		return err
	}

	return cmd.Wait()
}
