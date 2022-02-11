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
	"github.com/okteto/okteto/cmd/utils/displayer"
)

type ttyExecutor struct {
	displayer displayer.Displayer
}

var (
	spinnerChars = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	cursorUp     = "\x1b[1A"
	resetLine    = "\x1b[0G"
)

func newTTYExecutor() *ttyExecutor {
	return &ttyExecutor{}
}

func (e *ttyExecutor) display(command string) {
	e.displayer.Display(command)
}

func (e *ttyExecutor) cleanUp(err error) {
	if e.displayer != nil {
		e.displayer.CleanUp(err)
	}
}
