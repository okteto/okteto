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
	"os/exec"
)

type plainExecutorDisplayer struct {
	scanner *bufio.Scanner
}

func newPlainExecutorDisplayer() *plainExecutorDisplayer {
	return &plainExecutorDisplayer{}
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

func (*plainExecutorDisplayer) cleanUp(err error) {}

func (e *plainExecutorDisplayer) display(_ string) {
	for e.scanner.Scan() {
		line := e.scanner.Text()
		fmt.Println(line)
	}
}
