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

package executor

import (
	"os"
	"os/exec"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
)

//ManifestExecutor is the interface to execute a command
type ManifestExecutor interface {
	Execute(command model.DeployCommand, env []string) error
	CleanUp(err error)
}

//Executor implements ManifestExecutor with a executor displayer
type Executor struct {
	outputMode string
	displayer  executorDisplayer
}

type executorDisplayer interface {
	display(command string)
	startCommand(cmd *exec.Cmd) error
	cleanUp(err error)
}

// NewExecutor returns a new executor
func NewExecutor(output string) *Executor {
	var displayer executorDisplayer
	switch output {
	case oktetoLog.TTYFormat:
		displayer = newTTYExecutor()
	case oktetoLog.PlainFormat:
		displayer = newPlainExecutor()
	case oktetoLog.JSONFormat:
		displayer = newJSONExecutor()
	default:
		displayer = newTTYExecutor()
	}
	return &Executor{
		outputMode: output,
		displayer:  displayer,
	}
}

// Execute executes the specified command adding `env` to the execution environment
func (e *Executor) Execute(cmdInfo model.DeployCommand, env []string) error {

	cmd := exec.Command("bash", "-c", cmdInfo.Command)
	cmd.Env = append(os.Environ(), env...)

	if err := e.displayer.startCommand(cmd); err != nil {
		return err
	}

	e.displayer.display(cmdInfo.Name)

	err := cmd.Wait()

	e.CleanUp(err)
	return err
}

// CleanUp cleans the execution lines
func (e *Executor) CleanUp(err error) {
	if e.displayer != nil {
		e.displayer.cleanUp(err)
	}
}

func startCommand(cmd *exec.Cmd) error {
	return cmd.Start()
}
