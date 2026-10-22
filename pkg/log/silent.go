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

package log

import (
	"bytes"
	"fmt"
	"io"

	"github.com/sirupsen/logrus"
)

// SilentWriter writes into an in-memory buffer instead of the terminal
type SilentWriter struct {
	out    *logrus.Logger
	buffer io.ReadWriter
	file   *logrus.Entry
}

// newSilentWriter creates a new SilentWriter
func newSilentWriter(out *logrus.Logger, file *logrus.Entry) *SilentWriter {
	return &SilentWriter{
		out:    out,
		buffer: bytes.NewBuffer(nil),
		file:   file,
	}
}

// Debug writes a debug-level log
func (w *SilentWriter) Debug(args ...interface{}) {
	if log.file != nil {
		log.file.Debug(args...)
	}
}

// Debugf writes a debug-level log with a format
func (w *SilentWriter) Debugf(format string, args ...interface{}) {
	if log.file != nil {
		log.file.Debugf(format, args...)
	}
}

// Info writes an info-level log
func (w *SilentWriter) Info(args ...interface{}) {
	if log.file != nil {
		log.file.Info(args...)
	}
}

// Infof writes an info-level log with a format
func (*SilentWriter) Infof(format string, args ...interface{}) {
	if log.file != nil {
		log.file.Infof(format, args...)
	}
}

// Error writes an error-level log
func (w *SilentWriter) Error(args ...interface{}) {
	w.out.Error(args...)
	if log.file != nil {
		log.file.Error(args...)
	}
}

// Errorf writes an error-level log with a format
func (w *SilentWriter) Errorf(format string, args ...interface{}) {
	w.out.Errorf(format, args...)
	if log.file != nil {
		log.file.Errorf(format, args...)
	}
}

// Fatalf writes an error-level log with a format
func (w *SilentWriter) Fatalf(format string, args ...interface{}) {
	if log.file != nil {
		log.file.Errorf(format, args...)
	}

	w.out.Fatalf(format, args...)
}

// Green writes a line in green
func (w *SilentWriter) Green(format string, args ...interface{}) {
	w.FPrintln(w.buffer, fmt.Sprintf(format, args...))
}

// Yellow writes a line in yellow
func (w *SilentWriter) Yellow(format string, args ...interface{}) {
	w.FPrintln(w.buffer, fmt.Sprintf(format, args...))
}

// Success prints a message prefixed with 'SUCCESS:'
func (w *SilentWriter) Success(format string, args ...interface{}) {
	w.Fprintf(w.buffer, "SUCCESS: %s\n", fmt.Sprintf(format, args...))
}

// Information prints a message prefixed with 'INFO:'
func (w *SilentWriter) Information(format string, args ...interface{}) {
	w.Fprintf(w.buffer, "INFO: %s\n", fmt.Sprintf(format, args...))
}

// Question prints a message with the question symbol first, and the text in magenta
func (w *SilentWriter) Question(format string, args ...interface{}) error {
	w.Fprintf(w.buffer, "%s %s", questionSymbol, fmt.Sprintf(format, args...))
	return nil
}

// Warning prints a message prefixed with 'WARNING:'
func (w *SilentWriter) Warning(format string, args ...interface{}) {
	w.Fprintf(w.buffer, "WARNING: %s\n", fmt.Sprintf(format, args...))
}

// FWarning prints a message prefixed with 'WARNING:' into a specific writer
func (w *SilentWriter) FWarning(writer io.Writer, format string, args ...interface{}) {
	w.Fprintf(writer, "WARNING: %s\n", fmt.Sprintf(format, args...))
}

// Hint prints a message as plain text
func (w *SilentWriter) Hint(format string, args ...interface{}) {
	w.Fprintf(w.buffer, "%s\n", fmt.Sprintf(format, args...))
}

// Fail prints a message prefixed with 'ERROR:'
func (w *SilentWriter) Fail(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.out.Info(msg)
	w.Fprintf(w.out.Out, "ERROR: %s\n", fmt.Sprintf(format, args...))
}

// Println writes a line with colors
func (w *SilentWriter) Println(args ...interface{}) {
	msg := fmt.Sprint(args...)
	log.out.Info(msg)
	w.FPrintln(w.buffer, args...)
}

// Fprintf prints a line with format
func (w *SilentWriter) Fprintf(writer io.Writer, format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	fmt.Fprint(writer, msg)
}

// FPrintln prints a line to the given writer
func (w *SilentWriter) FPrintln(writer io.Writer, args ...interface{}) {
	msg := fmt.Sprint(args...)
	fmt.Fprintln(writer, msg)
}

// Print writes a line with colors
func (w *SilentWriter) Print(args ...interface{}) {
	msg := fmt.Sprint(args...)
	fmt.Fprint(w.buffer, msg)
}

// Printf writes a line with format
func (w *SilentWriter) Printf(format string, a ...interface{}) {
	w.Fprintf(w.buffer, format, a...)
}

// IsInteractive checks if the writer is interactive
func (*SilentWriter) IsInteractive() bool {
	return false
}

// AddToBuffer logs into the buffer but does not print anything
func (*SilentWriter) AddToBuffer(level, format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	log.writeRawToBuffer(msg)
}

// Write logs into the buffer but does not print anything
func (w *SilentWriter) Write(p []byte) (n int, err error) {
	return w.buffer.Write(p)
}

// PrintlnWithStage writes a line into the buffer. The silent writer doesn't record stages
func (w *SilentWriter) PrintlnWithStage(_, msg string) {
	log.out.Info(msg)
	w.FPrintln(w.buffer, msg)
}

// SuccessWithStage writes a success message into the buffer. The silent writer doesn't record stages
func (w *SilentWriter) SuccessWithStage(_, msg string) {
	w.Fprintf(w.buffer, "SUCCESS: %s\n", msg)
}

// WarningWithStage writes a warning message into the buffer. The silent writer doesn't record stages
func (w *SilentWriter) WarningWithStage(_, msg string) {
	w.Fprintf(w.buffer, "WARNING: %s\n", msg)
}
