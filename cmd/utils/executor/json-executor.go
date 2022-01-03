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

package executor

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type jsonMessage struct {
	Level     string `json:"level"`
	Stage     string `json:"stage"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

type jsonExecutorDisplayer struct {
	stdoutScanner *bufio.Scanner
	stderrScanner *bufio.Scanner
}

func newJsonExecutorDisplayer() *jsonExecutorDisplayer {
	return &jsonExecutorDisplayer{}
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
	}()

	go func() {
		for e.stderrScanner.Scan() {
			line := e.stderrScanner.Text()
			level := "error"
			messageStruct := jsonMessage{
				Level:     level,
				Message:   line,
				Stage:     command,
				Timestamp: time.Now().Unix(),
			}
			message, _ := json.Marshal(messageStruct)
			fmt.Println(string(message))
		}
	}()
}

func (*jsonExecutorDisplayer) cleanUp() {

}

func isErrorLine(text string) bool {
	return strings.HasPrefix(text, " x ")
}
