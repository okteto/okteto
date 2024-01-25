// Copyright 2023 The Okteto Authors
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

package io

import (
	"os"

	oktetoLog "github.com/okteto/okteto/pkg/log"
)

// Controller manages the input and output for the CLI
type Controller struct {
	in  *InputController
	out *OutputController
	*oktetoLogger
}

// NewIOController returns a new input/output controller
func NewIOController() *Controller {
	ioController := &Controller{
		out:          newOutputController(os.Stdout),
		in:           newInputController(os.Stdin),
		oktetoLogger: newOktetoLogger(),
	}
	return ioController
}

// In is utilized to collect user input for operations that require user interaction or data entry.
// When 'in' is used, the program will pause and wait for the user to provide input.
// This input is essential for executing specific actions, and the program proceeds once the user provides the necessary data or response.
func (ioc *Controller) In() *InputController {
	return ioc.in
}

// Out is used for displaying information to the user regardless of the log level set.
// It will always be shown to the user, regardless of whether the log level is 'info', 'debug', 'warning', or any other level.
func (ioc *Controller) Out() *OutputController {
	return ioc.out
}

// Logger is used for recording and categorizing log messages at different levels (e.g., info, debug, warning).
// These log messages can be filtered based on the log level set by the user.
// Messages with log levels lower than the user-defined log level will not be displayed to the user.
func (ioc *Controller) Logger() *oktetoLogger {
	return ioc.oktetoLogger
}

// SetOutputFormat sets the output format for the CLI. We need that the logger and the output generates the same
// type of messages, so we don't end up mixing formats like json and tty.
func (ioc *Controller) SetOutputFormat(output string) {
	ioc.oktetoLogger.SetOutputFormat(output)
	ioc.out.SetOutputFormat(output)
}

// SetStage sets the current stage where the CLI is performing.
func (ioc *Controller) SetStage(stage string) {
	ioc.oktetoLogger.SetStage(stage)
	ioc.out.SetStage(stage)
	oktetoLog.SetStage(stage)
}
