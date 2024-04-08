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
	"fmt"
	"io"
	"strings"

	"github.com/okteto/okteto/pkg/env"
)

// OutputController manages the output for the CLI
type OutputController struct {
	out io.Writer

	formatter formatter
	decorator decorator

	spinner OktetoSpinner
}

// newOutputController returns a new logger that writes to stdout
func newOutputController(out io.Writer) *OutputController {
	return &OutputController{
		out:       out,
		formatter: newTTYFormatter(),
		decorator: newTTYDecorator(),
	}
}

// SetOutputFormat sets the output format
func (oc *OutputController) SetOutputFormat(output string) {
	switch output {
	case "tty":
		oc.formatter = newTTYFormatter()
		oc.decorator = newTTYDecorator()
	case "plain":
		oc.formatter = newPlainFormatter()
		oc.decorator = newPlainDecorator()
	case "json":
		oc.formatter = newJSONFormatter()
		oc.decorator = newPlainDecorator()
	default:
		oc.formatter = newTTYFormatter()
		oc.decorator = newTTYDecorator()
	}
}

// Println prints a line into stdout
func (oc *OutputController) Println(args ...any) {
	msg := fmt.Sprint(args...)
	bytes, err := oc.formatter.format(msg)
	if err != nil {
		return
	}
	if oc.spinner != nil && oc.spinner.isActive() {
		oc.spinner.Stop()
		defer oc.Spinner(oc.spinner.getMessage()).Start()
	}
	fmt.Fprintln(oc.out, string(bytes))
}

// Print prints a line into stdout without a new line at the end
func (oc *OutputController) Print(args ...any) {
	msg := fmt.Sprint(args...)
	bytes, err := oc.formatter.format(msg)
	if err != nil {
		return
	}
	if oc.spinner != nil && oc.spinner.isActive() {
		oc.spinner.Stop()
		defer oc.Spinner(oc.spinner.getMessage()).Start()
	}
	fmt.Fprint(oc.out, string(bytes))
}

// Printf prints a line into stdout with a format
func (oc *OutputController) Printf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	bytes, err := oc.formatter.format(msg)
	if err != nil {
		return
	}
	if oc.spinner != nil && oc.spinner.isActive() {
		oc.spinner.Stop()
		defer oc.Spinner(oc.spinner.getMessage()).Start()
	}
	fmt.Fprint(oc.out, string(bytes))
}

// Infof prints a information message to the user
func (oc *OutputController) Infof(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	msg = oc.decorator.Information(msg)
	bytes, err := oc.formatter.format(msg)
	if err != nil {
		return
	}
	if oc.spinner != nil && oc.spinner.isActive() {
		oc.spinner.Stop()
		defer oc.Spinner(oc.spinner.getMessage()).Start()
	}
	fmt.Fprint(oc.out, string(bytes))
}

// Success prints a success message to the user
func (oc *OutputController) Success(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	msg = oc.decorator.Success(msg)
	bytes, err := oc.formatter.format(msg)
	if err != nil {
		return
	}
	if oc.spinner != nil && oc.spinner.isActive() {
		oc.spinner.Stop()
		defer oc.Spinner(oc.spinner.getMessage()).Start()
	}
	fmt.Fprint(oc.out, string(bytes))
}

// Warning prints a warning message to the user
func (oc *OutputController) Warning(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	msg = oc.decorator.Warning(msg)
	bytes, err := oc.formatter.format(msg)
	if err != nil {
		return
	}
	if oc.spinner != nil && oc.spinner.isActive() {
		oc.spinner.Stop()
		defer oc.Spinner(oc.spinner.getMessage()).Start()
	}
	fmt.Fprint(oc.out, string(bytes))
}

// SetStage sets the stage of the logger if it's json
func (oc *OutputController) SetStage(stage string) {
	if v, ok := oc.formatter.(*jsonFormatter); ok {
		v.SetStage(stage)
	}
}

// Spinner returns a spinner
func (oc *OutputController) Spinner(msg string) OktetoSpinner {
	if oc.spinner != nil {
		if oc.spinner.getMessage() == msg {
			return oc.spinner
		}
		oc.spinner.Stop()
	}

	disableSpinner := env.LoadBoolean(OktetoDisableSpinnerEnvVar)

	_, isTTY := oc.formatter.(*ttyFormatter)
	if isTTY && !disableSpinner {
		oc.spinner = newTTYSpinner(msg)
	} else {
		oc.spinner = newNoSpinner(msg, oc)
	}
	return oc.spinner
}

// Write logs into the buffer but does not print anything
func (oc *OutputController) Write(p []byte) (n int, err error) {
	msg := string(p)
	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}
	bytes, err := oc.formatter.format(msg)
	if err != nil {
		return
	}

	return oc.out.Write(bytes)
}
