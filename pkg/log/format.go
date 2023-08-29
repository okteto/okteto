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

import "io"

// OktetoWriter implements the interface of the writers
type OktetoWriter interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})

	Info(args ...interface{})
	Infof(format string, args ...interface{})

	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Fail(format string, args ...interface{})
	Fatalf(format string, args ...interface{})

	Yellow(format string, args ...interface{})
	Green(format string, args ...interface{})

	Success(format string, args ...interface{})
	Information(format string, args ...interface{})
	Question(format string, args ...interface{}) error
	Warning(format string, args ...interface{})
	FWarning(w io.Writer, format string, args ...interface{})
	Hint(format string, args ...interface{})

	Println(args ...interface{})
	FPrintln(w io.Writer, args ...interface{})
	Print(args ...interface{})
	Fprintf(w io.Writer, format string, a ...interface{})
	Printf(format string, a ...interface{})

	IsInteractive() bool
	AddToBuffer(level, format string, a ...interface{})

	Write(p []byte) (n int, err error)
}

const (
	// TTYFormat represents a tty logger
	TTYFormat string = "tty"
	// PlainFormat represents a plain logger
	PlainFormat string = "plain"
	// JSONFormat represents a json logger
	JSONFormat string = "json"
	// SilentFormat represents a silent logger
	SilentFormat string = "silent"
)

func (l *logger) getWriter(format string) OktetoWriter {
	switch format {
	case TTYFormat:
		l.outputMode = TTYFormat
		return newTTYWriter(l.out, l.file)
	case PlainFormat:
		l.outputMode = PlainFormat
		return newPlainWriter(l.out, l.file)
	case JSONFormat:
		l.outputMode = JSONFormat
		l.out.SetFormatter(&JSONLogFormat{})
		return newJSONWriter(l.out, l.file)
	case SilentFormat:
		l.outputMode = SilentFormat
		return newSilentWriter(l.out, l.file)
	default:
		Debugf("could not load %s. Callback to 'tty'", format)
		l.outputMode = TTYFormat
		return newTTYWriter(l.out, l.file)
	}

}
