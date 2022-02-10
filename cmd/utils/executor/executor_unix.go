//go:build !windows
// +build !windows

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
	"bufio"
	"os"
	"os/exec"

	"github.com/creack/pty"
	"github.com/manifoldco/promptui/screenbuf"
)

func (e *ttyExecutor) startCommand(cmd *exec.Cmd) error {
	e.screenbuf = screenbuf.New(os.Stdout)

	stderrReader, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	e.stderrScanner = bufio.NewScanner(stderrReader)

	f, err := pty.Start(cmd)
	if err != nil {
		return err
	}

	e.stdoutScanner = bufio.NewScanner(f)
	return nil
}
