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
	"fmt"
	"io"
	"os/exec"
)

type plainExecutorDisplayer struct {
	cmdInfo *commandInfo
}

func newPlainExecutorDisplayer() *plainExecutorDisplayer {
	return &plainExecutorDisplayer{}
}

func (plainExecutorDisplayer) startCommand(cmd *exec.Cmd) (io.Reader, error) {
	return startCommand(cmd)
}

func (*plainExecutorDisplayer) cleanUp() {}

func (e *plainExecutorDisplayer) addCommandInfo(cmdInfo *commandInfo) {
	e.cmdInfo = cmdInfo
}

func (*plainExecutorDisplayer) display(scanner *bufio.Scanner) {
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Println(line)
	}
}
