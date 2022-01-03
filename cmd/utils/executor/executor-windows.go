//go:build windows
// +build windows

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

// This file is needed because we can't check if a windows terminal supports tty
// This w

package executor

import (
	"io"
	"os/exec"
)

func (ttyExecutorDisplayer) startCommand(cmd *exec.Cmd) (io.Reader, error) {
	e.screenbuf = screenbuf.New(os.Stdout)

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
