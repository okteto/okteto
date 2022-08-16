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

package log

import (
	"fmt"
	"io"
	"regexp"

	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
)

const ansi = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"

var ansiRegex = regexp.MustCompile(ansi)

// TTYWriter writes into a tty terminal
type TTYWriter struct {
	out  *logrus.Logger
	file *logrus.Entry
}

// newTTYWriter creates a new ttyWriter
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
	log.spinner.hold()
	w.FPrintln(w.out.Out, greenString(format, args...))
	log.spinner.unhold()
}

// Yellow writes a line in yellow
func (w *TTYWriter) Yellow(format string, args ...interface{}) {
	log.out.Infof(format, args...)
	log.spinner.hold()
	w.FPrintln(w.out.Out, yellowString(format, args...))
	log.spinner.unhold()
}

// Success prints a message with the success symbol first, and the text in green
func (w *TTYWriter) Success(format string, args ...interface{}) {
	log.out.Infof(format, args...)
	log.spinner.hold()
	w.Fprintf(w.out.Out, "%s %s\n", coloredSuccessSymbol, greenString(format, args...))
	log.spinner.unhold()
}

// Information prints a message with the information symbol first, and the text in blue
func (w *TTYWriter) Information(format string, args ...interface{}) {
	log.out.Infof(format, args...)
	log.spinner.hold()
	w.Fprintf(w.out.Out, "%s %s\n", coloredInformationSymbol, blueString(format, args...))
	log.spinner.unhold()
}

// Question prints a message with the question symbol first, and the text in magenta
func (w *TTYWriter) Question(format string, args ...interface{}) error {
	log.out.Infof(format, args...)
	log.spinner.hold()
	w.Fprintf(w.out.Out, "%s %s", coloredQuestionSymbol, color.MagentaString(format, args...))
	log.spinner.unhold()
	return nil
}

// Warning prints a message with the warning symbol first, and the text in yellow
func (w *TTYWriter) Warning(format string, args ...interface{}) {
	log.out.Infof(format, args...)
	log.spinner.hold()
	w.Fprintf(w.out.Out, "%s %s\n", coloredWarningSymbol, yellowString(format, args...))
	log.spinner.unhold()
}

// FWarning prints a message with the warning symbol first, and the text in yellow into an specific writer
func (w *TTYWriter) FWarning(writer io.Writer, format string, args ...interface{}) {
	log.out.Infof(format, args...)
	log.spinner.hold()
	w.Fprintf(writer, "%s %s\n", coloredWarningSymbol, yellowString(format, args...))
	log.spinner.unhold()
}

// Hint prints a message with the text in blue
func (w *TTYWriter) Hint(format string, args ...interface{}) {
	log.out.Infof(format, args...)
	log.spinner.hold()
	w.Fprintf(w.out.Out, "%s\n", blueString(format, args...))
	log.spinner.unhold()
}

// Fail prints a message with the error symbol first, and the text in red
func (w *TTYWriter) Fail(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.out.Info(msg)
	log.spinner.hold()
	w.Fprintf(w.out.Out, "%s %s\n", coloredErrorSymbol, redString(format, args...))
	log.spinner.unhold()
	if msg != "" {
		msg = convertToJSON(ErrorLevel, log.stage, msg)
		if msg != "" {
			log.buf.WriteString(msg)
			log.buf.WriteString("\n")
		}
	}
}

// Println writes a line with colors
func (w *TTYWriter) Println(args ...interface{}) {
	log.out.Info(args...)
	log.spinner.hold()
	w.FPrintln(w.out.Out, args...)
	log.spinner.unhold()
}

// Fprintf prints a line with format
func (w *TTYWriter) Fprintf(writer io.Writer, format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	fmt.Fprint(writer, msg)
	if msg != "" && writer == w.out.Out {
		msg = convertToJSON(InfoLevel, log.stage, msg)
		if msg != "" {
			log.buf.WriteString(msg)
			log.buf.WriteString("\n")
		}
	}

}

// FPrintln prints a line with format
func (w *TTYWriter) FPrintln(writer io.Writer, args ...interface{}) {
	msg := fmt.Sprint(args...)
	fmt.Fprintln(writer, msg)
	if msg != "" && writer == w.out.Out {
		msg = convertToJSON(InfoLevel, log.stage, msg)
		if msg != "" {
			log.buf.WriteString(msg)
			log.buf.WriteString("\n")
		}
	}

}

// Print writes a line with colors
func (w *TTYWriter) Print(args ...interface{}) {
	msg := fmt.Sprint(args...)
	fmt.Fprint(w.out.Out, args...)
	if msg != "" {
		msg = convertToJSON(ErrorLevel, log.stage, msg)
		if msg != "" {
			log.buf.WriteString(msg)
			log.buf.WriteString("\n")
		}
	}

}

// Printf writes a line with format
func (w *TTYWriter) Printf(format string, a ...interface{}) {
	log.spinner.hold()
	w.Fprintf(w.out.Out, format, a...)
	log.spinner.unhold()
}

// IsInteractive checks if the writer is interactive
func (*TTYWriter) IsInteractive() bool {
	return true
}

// AddToBuffer logs into the buffer but does not print anything
func (*TTYWriter) AddToBuffer(level, format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	if msg != "" {
		msg = convertToJSON(level, log.stage, msg)
		if msg != "" {
			log.buf.WriteString(msg)
			log.buf.WriteString("\n")
		}
	}
}

// AddToBuffer logs into the buffer but does not print anything
func (w *TTYWriter) Write(p []byte) (n int, err error) {
	return w.out.Out.Write(p)
}
