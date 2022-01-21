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

package log

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
)

//TTYWriter writes into a tty terminal
type TTYWriter struct {
	out  *logrus.Logger
	file *logrus.Entry
}

//newTTYWriter creates a new ttyWriter
func newTTYWriter(out *logrus.Logger, file *logrus.Entry) *TTYWriter {
	return &TTYWriter{
		out:  out,
		file: file,
	}
}

// Debug writes a debug-level log
func (w *TTYWriter) Debug(args ...interface{}) {
	w.out.Debug(args...)
	if log.file != nil {
		log.file.Debug(args...)
	}
}

// Debugf writes a debug-level log with a format
func (*TTYWriter) Debugf(format string, args ...interface{}) {
	log.out.Debugf(format, args...)
	if log.file != nil {
		log.file.Debugf(format, args...)
	}
}

// Info writes a info-level log
func (*TTYWriter) Info(args ...interface{}) {
	log.out.Info(args...)
	if log.file != nil {
		log.file.Info(args...)
	}
}

// Infof writes a info-level log with a format
func (*TTYWriter) Infof(format string, args ...interface{}) {
	log.out.Infof(format, args...)
	if log.file != nil {
		log.file.Infof(format, args...)
	}
}

// Error writes a error-level log
func (*TTYWriter) Error(args ...interface{}) {
	log.out.Error(args...)
	if log.file != nil {
		log.file.Error(args...)
	}
}

// Errorf writes a error-level log with a format
func (*TTYWriter) Errorf(format string, args ...interface{}) {
	log.out.Errorf(format, args...)
	if log.file != nil {
		log.file.Errorf(format, args...)
	}
}

// Fatalf writes a error-level log with a format
func (*TTYWriter) Fatalf(format string, args ...interface{}) {
	if log.file != nil {
		log.file.Errorf(format, args...)
	}

	log.out.Fatalf(format, args...)
}

// Green writes a line in green
func (w *TTYWriter) Green(format string, args ...interface{}) {
	log.out.Infof(format, args...)
	w.Fprintln(greenString(format, args...))
}

// Yellow writes a line in yellow
func (w *TTYWriter) Yellow(format string, args ...interface{}) {
	log.out.Infof(format, args...)
	w.Fprintln(yellowString(format, args...))
}

// Success prints a message with the success symbol first, and the text in green
func (w *TTYWriter) Success(format string, args ...interface{}) {
	log.out.Infof(format, args...)
	w.Fprintf("%s %s\n", coloredSuccessSymbol, greenString(format, args...))
}

// Information prints a message with the information symbol first, and the text in blue
func (w *TTYWriter) Information(format string, args ...interface{}) {
	log.out.Infof(format, args...)
	w.Fprintf("%s %s\n", coloredInformationSymbol, blueString(format, args...))
}

// Question prints a message with the question symbol first, and the text in magenta
func (w *TTYWriter) Question(format string, args ...interface{}) error {
	log.out.Infof(format, args...)
	w.Fprintf("%s %s", coloredQuestionSymbol, color.MagentaString(format, args...))
	return nil
}

// Warning prints a message with the warning symbol first, and the text in yellow
func (w *TTYWriter) Warning(format string, args ...interface{}) {
	log.out.Infof(format, args...)
	w.Fprintf("%s %s\n", coloredWarningSymbol, yellowString(format, args...))
}

// Hint prints a message with the text in blue
func (w *TTYWriter) Hint(format string, args ...interface{}) {
	log.out.Infof(format, args...)
	w.Fprintf("%s\n", blueString(format, args...))
}

// Fail prints a message with the error symbol first, and the text in red
func (w *TTYWriter) Fail(format string, args ...interface{}) {
	log.out.Infof(format, args...)
	w.Fprintf("%s %s\n", coloredErrorSymbol, redString(format, args...))
}

// Println writes a line with colors
func (w *TTYWriter) Println(args ...interface{}) {
	log.out.Info(args...)
	w.Fprintln(args...)
}

// Fprintf prints a line with format
func (w *TTYWriter) Fprintf(format string, a ...interface{}) {
	fmt.Fprintf(w.out.Out, format, a...)
}

// Fprintln prints a line with format
func (w *TTYWriter) Fprintln(args ...interface{}) {
	fmt.Fprintln(w.out.Out, args...)
}

// Print writes a line with colors
func (w *TTYWriter) Print(args ...interface{}) {
	fmt.Fprint(w.out.Out, args...)
}

//Printf writes a line with format
func (w *TTYWriter) Printf(format string, a ...interface{}) {
	w.Fprintf(format, a...)
}

//IsInteractive checks if the writer is interactive
func (*TTYWriter) IsInteractive() bool {
	return true
}
