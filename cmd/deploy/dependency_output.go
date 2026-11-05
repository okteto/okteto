// Copyright 2026 The Okteto Authors
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

package deploy

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"

	"github.com/fatih/color"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
)

// dependenciesStage is the log stage of the output of every dependency. Like the other stages that deploy or destroy
// dependencies, it starts with an information line with its name
const dependenciesStage = "Deploying dependencies"

// dependencyMark is how the logger marks the level of a message: a symbol in a terminal and in JSON, and a label in
// plain output
type dependencyMark struct {
	badge  *color.Color
	text   func(format string, a ...interface{}) string
	symbol string
	label  string
	level  string
}

var (
	successMark = dependencyMark{symbol: successSymbol(), label: "SUCCESS:", level: oktetoLog.InfoLevel, badge: color.New(color.BgGreen, color.FgBlack), text: color.New(color.FgGreen).SprintfFunc()}
	warningMark = dependencyMark{symbol: " ! ", label: "WARNING:", level: oktetoLog.WarningLevel, badge: color.New(color.BgHiYellow, color.FgBlack), text: color.New(color.FgHiYellow).SprintfFunc()}
	errorMark   = dependencyMark{symbol: " x ", label: "ERROR:", level: oktetoLog.ErrorLevel, badge: color.New(color.BgHiRed, color.FgBlack), text: color.New(color.FgHiRed).SprintfFunc()}
)

// successSymbol is the symbol the logger uses for a success
func successSymbol() string {
	if runtime.GOOS == "windows" {
		return " + "
	}
	return " ✓ "
}

// print prints msg marked as the logger would, with the prefix before the mark
func (m dependencyMark) print(prefix, msg string) {
	switch oktetoLog.GetOutputFormat() {
	case oktetoLog.JSONFormat:
		// it prints the line too, with the level of the message
		oktetoLog.AddToBuffer(m.level, "%s %s %s", prefix, m.symbol, msg)
	case oktetoLog.TTYFormat:
		oktetoLog.Println(fmt.Sprintf("%s %s %s", prefix, m.badge.Sprint(m.symbol), m.text("%s", msg)))
		m.recordError(prefix, msg)
	default:
		oktetoLog.Println(fmt.Sprintf("%s %s %s", prefix, m.label, msg))
		m.recordError(prefix, msg)
	}
}

// recordError stores an error with its level in the output buffer, as the logger does when it prints one
func (m dependencyMark) recordError(prefix, msg string) {
	if m.level == oktetoLog.ErrorLevel {
		oktetoLog.AddToBuffer(oktetoLog.ErrorLevel, "%s %s", prefix, msg)
	}
}

// printLine prints a log line of a dependency with its prefix
func printLine(prefix, line string) {
	oktetoLog.Println(fmt.Sprintf("%s %s", prefix, line))
}

// dependencyOutputMu serializes the lines of the dependencies deployed in parallel, so that the lines of a message
// are printed together. It is global because it guards a process-wide resource: the global logger
var dependencyOutputMu sync.Mutex

// dependencyOutput is the output of the pipeline deploy of a dependency, printed through the global logger under
// the dependencies stage. Dependencies are deployed in parallel and their lines are interleaved, so every line is
// prefixed with the name of its dependency, and the spinner messages are printed once, as lines. A dependency that
// is only scheduled shows its result, without the messages of its deploy request
type dependencyOutput struct {
	name    string
	awaited bool
}

func newDependencyOutput(name string, awaited bool) *dependencyOutput {
	return &dependencyOutput{name: name, awaited: awaited}
}

// Spinner prints the message once: a spinner can't represent several dependencies at the same time
func (o *dependencyOutput) Spinner(msg string) {
	if o.awaited {
		o.print(printLine, msg)
	}
}
func (*dependencyOutput) StartSpinner()         {}
func (*dependencyOutput) StopSpinner()          {}
func (o *dependencyOutput) Success(msg string)  { o.print(successMark.print, msg) }
func (o *dependencyOutput) Warning(msg string)  { o.print(warningMark.print, msg) }
func (o *dependencyOutput) Println(line string) { o.print(printLine, line) }

// finish prints the error of the dependency, if it failed. An interruption is not an error of the dependency:
// the deploy reports it
func (o *dependencyOutput) finish(err error) {
	if err != nil && !errors.Is(err, oktetoErrors.ErrIntSig) {
		o.print(errorMark.print, err.Error())
	}
}

// print prefixes every line of msg with the name of the dependency and prints them together
func (o *dependencyOutput) print(printLine func(prefix, line string), msg string) {
	dependencyOutputMu.Lock()
	defer dependencyOutputMu.Unlock()
	prefix := fmt.Sprintf("[%s]", o.name)
	for _, line := range strings.Split(strings.TrimRight(msg, "\r\n"), "\n") {
		printLine(prefix, strings.TrimRight(line, "\r"))
	}
}
