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
	"io"
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
	cmdInfo *commandInfo
}

func newJsonExecutorDisplayer() *plainExecutorDisplayer {
	return &plainExecutorDisplayer{}
}

func (*jsonExecutorDisplayer) startCommand(cmd *exec.Cmd) (io.Reader, error) {
	return startCommand(cmd)
}

func (e *jsonExecutorDisplayer) addCommandInfo(cmdInfo *commandInfo) {
	e.cmdInfo = cmdInfo
}

func (e *jsonExecutorDisplayer) display(scanner *bufio.Scanner) {
	for scanner.Scan() {
		line := scanner.Text()
		level := "info"
		if isErrorLine(line) {
			level = "error"
		}
		messageStruct := jsonMessage{
			Level:     level,
			Message:   line,
			Stage:     e.cmdInfo.command,
			Timestamp: time.Now().Unix(),
		}
		message, _ := json.Marshal(messageStruct)
		fmt.Println(string(message))
	}
}
func (*jsonExecutorDisplayer) cleanUp() {

}

func isErrorLine(text string) bool {
	return strings.HasPrefix(text, " x ")
}
