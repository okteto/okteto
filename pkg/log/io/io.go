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
	"log/slog"
	"os"
)

// IOController manages the input and output for the CLI
type IOController struct {
	in     *InputController
	out    *OutputController
	logger *oktetoLogger
}

// NewIOController returns a new input/output controller
func NewIOController() *IOController {
	ioController := &IOController{
		out:    newOutputController(os.Stdout),
		in:     newInputController(os.Stdin),
		logger: newOktetoLogger(),
	}
	return ioController
}

// In returns the logger for the input
func (ioc *IOController) In() *InputController {
	return ioc.in
}

// Out returns the logger for the output
func (ioc *IOController) Out() *OutputController {
	return ioc.out
}

// Logger returns the logger
func (ioc *IOController) Logger() *slog.Logger {
	return ioc.logger.Logger
}

func (ioc *IOController) SetLevel(lvl string) {
	ioc.logger.SetLevel(lvl)
}

func (ioc *IOController) SetOutputFormat(output string) {
	ioc.logger.SetOutputFormat(output)
	ioc.out.SetOutputFormat(output)
}

func (ioc *IOController) SetStage(stage string) {
	ioc.logger.SetStage(stage)
	ioc.out.SetStage(stage)
}
