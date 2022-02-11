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
	"os/exec"

	"github.com/okteto/okteto/cmd/utils/displayer"
	oktetoLog "github.com/okteto/okteto/pkg/log"
)

type jsonExecutor struct {
	displayer displayer.Displayer
}

func newJSONExecutor() *jsonExecutor {
	return &jsonExecutor{}
}

func (e *jsonExecutor) startCommand(cmd *exec.Cmd) error {
	stdoutReader, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	stderrReader, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	e.displayer = displayer.NewDisplayer(oktetoLog.GetOutputFormat(), stdoutReader, stderrReader)
	return startCommand(cmd)
}

func (e *jsonExecutor) display(cmd string) {
	e.displayer.Display(cmd)
}
func (e *jsonExecutor) cleanUp(err error) {
	e.displayer.CleanUp(err)
}
