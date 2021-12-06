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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/manifoldco/promptui/screenbuf"
)

type ManifestExecutor interface {
	Execute(command string, env []string) error
}

type Executor struct {
	outputMode string
	displayer  executorDisplayer
}

type executorDisplayer interface {
	display(scanner *bufio.Scanner, command string, sb *screenbuf.ScreenBuf)
	startCommand(cmd *exec.Cmd) (io.Reader, error)
}

type plainExecutorDisplayer struct{}
type jsonExecutorDisplayer struct{}

type jsonMessage struct {
	Level     string `json:"level"`
	Stage     string `json:"stage"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

// NewExecutor returns a new executor
func NewExecutor(output string) *Executor {
	var displayer executorDisplayer
	switch output {
	case "plain":
		displayer = plainExecutorDisplayer{}
	case "json":
		displayer = jsonExecutorDisplayer{}
	default:
		displayer = plainExecutorDisplayer{}
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

	reader, err := e.displayer.startCommand(cmd)
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(reader)

	sb := screenbuf.New(os.Stdout)
	go e.displayer.display(scanner, command, sb)

	err = cmd.Wait()
	return err
}

func startCommand(cmd *exec.Cmd) (io.Reader, error) {
	reader, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return reader, nil
}

func (plainExecutorDisplayer) startCommand(cmd *exec.Cmd) (io.Reader, error) {
	return startCommand(cmd)
}

func (plainExecutorDisplayer) display(scanner *bufio.Scanner, _ string, _ *screenbuf.ScreenBuf) {
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Println(line)
	}
}

func (jsonExecutorDisplayer) startCommand(cmd *exec.Cmd) (io.Reader, error) {
	return startCommand(cmd)
}

func (jsonExecutorDisplayer) display(scanner *bufio.Scanner, command string, _ *screenbuf.ScreenBuf) {
	for scanner.Scan() {
		line := scanner.Text()
		level := "info"
		if isErrorLine(line) {
			level = "error"
		}
		messageStruct := jsonMessage{
			Level:     level,
			Message:   line,
			Stage:     command,
			Timestamp: time.Now().Unix(),
		}
		message, _ := json.Marshal(messageStruct)
		fmt.Println(string(message))
	}
}

func isErrorLine(text string) bool {
	return strings.HasPrefix(text, " x ")
}
