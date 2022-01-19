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

//OktetoWriter implements the interface of the writers
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
	Hint(format string, args ...interface{})

	Println(args ...interface{})
	Print(args ...interface{})
	Fprintf(format string, a ...interface{})
	Printf(format string, a ...interface{})

	IsInteractive() bool
}

const (
	ttyFormat   string = "tty"
	plainFormat string = "plain"
	jsonFormat  string = "json"
)

func (l *logger) getWriter(format string) OktetoWriter {
	switch format {
	case ttyFormat:
		l.outputMode = "tty"
		return newTTYWriter(l.out, l.file)
	case plainFormat:
		l.outputMode = "plain"
		return newPlainWriter(l.out, l.file)
	case jsonFormat:
		l.outputMode = "json"
		l.out.SetFormatter(&JSONLogFormat{})
		return newJSONWriter(l.out, l.file)
	default:
		l.outputMode = "tty"
		return newTTYWriter(l.out, l.file)
	}

}
