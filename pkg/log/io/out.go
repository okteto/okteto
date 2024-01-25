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
func (l *OutputController) Println(args ...any) {
	msg := fmt.Sprint(args...)
	bytes, err := l.formatter.format(msg)
	if err != nil {
		return
	}
	if l.spinner != nil && l.spinner.isActive() {
		l.spinner.Stop()
		defer l.Spinner(l.spinner.getMessage()).Start()
	}
	fmt.Fprintln(l.out, string(bytes))
}

// Print prints a line into stdout without a new line at the end
func (l *OutputController) Print(args ...any) {
	msg := fmt.Sprint(args...)
	bytes, err := l.formatter.format(msg)
	if err != nil {
		return
	}
	if l.spinner != nil && l.spinner.isActive() {
		l.spinner.Stop()
		defer l.Spinner(l.spinner.getMessage()).Start()
	}
	fmt.Fprint(l.out, string(bytes))
}

// Printf prints a line into stdout with a format
func (l *OutputController) Printf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	bytes, err := l.formatter.format(msg)
	if err != nil {
		return
	}
	if l.spinner != nil && l.spinner.isActive() {
		l.spinner.Stop()
		defer l.Spinner(l.spinner.getMessage()).Start()
	}
	fmt.Fprint(l.out, string(bytes))
}

// Infof prints a information message to the user
func (l *OutputController) Infof(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	msg = l.decorator.Information(msg)
	bytes, err := l.formatter.format(msg)
	if err != nil {
		return
	}
	if l.spinner != nil && l.spinner.isActive() {
		l.spinner.Stop()
		defer l.Spinner(l.spinner.getMessage()).Start()
	}
	fmt.Fprint(l.out, string(bytes))
}

// Success prints a success message to the user
func (l *OutputController) Success(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	msg = l.decorator.Success(msg)
	bytes, err := l.formatter.format(msg)
	if err != nil {
		return
	}
	if l.spinner != nil && l.spinner.isActive() {
		l.spinner.Stop()
		defer l.Spinner(l.spinner.getMessage()).Start()
	}
	fmt.Fprint(l.out, string(bytes))
}

// SetStage sets the stage of the logger if it's json
func (l *OutputController) SetStage(stage string) {
	if v, ok := l.formatter.(*jsonFormatter); ok {
		v.SetStage(stage)
	}
}

// Spinner returns a spinner
func (l *OutputController) Spinner(msg string) OktetoSpinner {
	if l.spinner != nil {
		if l.spinner.getMessage() == msg {
			return l.spinner
		}
		l.spinner.Stop()
	}

	disableSpinner := env.LoadBoolean(OktetoDisableSpinnerEnvVar)

	_, isTTY := l.formatter.(*ttyFormatter)
	if isTTY && !disableSpinner {
		l.spinner = newTTYSpinner(msg)
	} else {
		l.spinner = newNoSpinner(msg, l)
	}
	return l.spinner
}

// Write logs into the buffer but does not print anything
func (l *OutputController) Write(p []byte) (n int, err error) {
	msg := string(p)
	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}
	bytes, err := l.formatter.format(msg)
	if err != nil {
		return
	}

	return l.out.Write(bytes)
}
