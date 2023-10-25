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
)

// IOController manages the input and output for the CLI
type IOController struct {
	in  *InputController
	out *OutputController
	*oktetoLogger
}

// NewIOController returns a new input/output controller
func NewIOController() *IOController {
	ioController := &IOController{
		out:          newOutputController(os.Stdout),
		in:           newInputController(os.Stdin),
		oktetoLogger: newOktetoLogger(),
	}
	return ioController
}

// In returns the input reader
func (ioc *IOController) In() *InputController {
	return ioc.in
}

// Out returns the output writer
func (ioc *IOController) Out() *OutputController {
	return ioc.out
}

// Logger returns the logger
func (ioc *IOController) Logger() *oktetoLogger {
	return ioc.oktetoLogger
}

func (ioc *IOController) SetOutputFormat(output string) {
	ioc.oktetoLogger.SetOutputFormat(output)
	ioc.out.SetOutputFormat(output)
}

func (ioc *IOController) SetStage(stage string) {
	ioc.oktetoLogger.SetStage(stage)
	ioc.out.SetStage(stage)
}
