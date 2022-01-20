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
	"os"
	"os/exec"

	oktetoLog "github.com/okteto/okteto/pkg/log"
)

type ManifestExecutor interface {
	Execute(command string, env []string) error
}

type Executor struct {
	outputMode string
	displayer  executorDisplayer
}

type executorDisplayer interface {
	display(command string)
	startCommand(cmd *exec.Cmd) error
}

type plainExecutorDisplayer struct {
	scanner *bufio.Scanner
}
type jsonExecutorDisplayer struct {
	stdoutScanner *bufio.Scanner
	stderrScanner *bufio.Scanner
}

// NewExecutor returns a new executor
func NewExecutor(output string) *Executor {
	var displayer executorDisplayer
	switch output {
	case "plain":
		displayer = &plainExecutorDisplayer{}
	case "json":
		displayer = &jsonExecutorDisplayer{}
	default:
		displayer = &plainExecutorDisplayer{}
	}
	return &Executor{
		outputMode: output,
		displayer:  displayer,
	}
}

// Execute executes the specified command adding `env` to the execution environment
func (e *Executor) Execute(command string, env []string) error {

	cmd := exec.Command("bash", "-c", command)
	cmd.Env = append(os.Environ(), env...)

	if err := e.displayer.startCommand(cmd); err != nil {
		return err
	}

	go e.displayer.display(command)

	err := cmd.Wait()

	return err
}

func startCommand(cmd *exec.Cmd) error {
	return cmd.Start()
}

func (e *plainExecutorDisplayer) startCommand(cmd *exec.Cmd) error {

	reader, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout

	if err := startCommand(cmd); err != nil {
		return err
	}
	e.scanner = bufio.NewScanner(reader)
	return nil
}

func (e *plainExecutorDisplayer) display(_ string) {
	for e.scanner.Scan() {
		line := e.scanner.Text()
		oktetoLog.Println(line)
	}
}

func (e *jsonExecutorDisplayer) startCommand(cmd *exec.Cmd) error {
	stdoutReader, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	e.stdoutScanner = bufio.NewScanner(stdoutReader)

	stderrReader, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	e.stderrScanner = bufio.NewScanner(stderrReader)
	return startCommand(cmd)
}

func (e *jsonExecutorDisplayer) display(command string) {
	go func() {
		for e.stdoutScanner.Scan() {
			line := e.stdoutScanner.Text()

			oktetoLog.Println(line)
		}
	}()

	go func() {
		for e.stderrScanner.Scan() {
			line := e.stderrScanner.Text()
			oktetoLog.Fail(line)

		}
	}()
}
