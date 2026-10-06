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

package pipeline

import (
	"fmt"
	"strings"
	"sync"

	oktetoLog "github.com/okteto/okteto/pkg/log"
)

// deployOutput prints the messages and logs of a pipeline deploy
type deployOutput interface {
	Spinner(msg string)
	StartSpinner()
	StopSpinner()
	Success(msg string)
	Warning(msg string)
	Println(line string)
}

func newDeployOutput(opts *DeployOptions) deployOutput {
	if opts.PrefixLogs {
		return &prefixedDeployOutput{
			prefix:  opts.Name,
			stage:   DependencyStage(opts.Name),
			printer: globalLinePrinter(),
		}
	}
	return globalDeployOutput{}
}

// DependencyStage is the log stage of the messages of a dependency
func DependencyStage(name string) string {
	return fmt.Sprintf("Deploying dependency %s", name)
}

// linePrinter prints a single line keeping its log level, recorded under the given stage
type linePrinter struct {
	println func(stage, line string)
	success func(stage, line string)
	warning func(stage, line string)
}

func globalLinePrinter() linePrinter {
	return linePrinter{
		println: oktetoLog.PrintlnWithStage,
		success: oktetoLog.SuccessWithStage,
		warning: oktetoLog.WarningWithStage,
	}
}

// globalDeployOutput prints through the global logger, with spinner. It is the default output
type globalDeployOutput struct{}

func (globalDeployOutput) Spinner(msg string)  { oktetoLog.Spinner(msg) }
func (globalDeployOutput) StartSpinner()       { oktetoLog.StartSpinner() }
func (globalDeployOutput) StopSpinner()        { oktetoLog.StopSpinner() }
func (globalDeployOutput) Success(msg string)  { oktetoLog.Success("%s", msg) }
func (globalDeployOutput) Warning(msg string)  { oktetoLog.Warning("%s", msg) }
func (globalDeployOutput) Println(line string) { oktetoLog.Println(line) }

// prefixedOutputMu serializes the lines printed by pipelines deployed in parallel.
// It is global because it guards a process-wide resource: the global logger
var prefixedOutputMu sync.Mutex

// prefixedDeployOutput prints every message and log line as "[prefix] line", without spinner, recorded
// under its own stage. It is used when pipelines are deployed in parallel and their output is interleaved:
// the global stage belongs to the main deploy
type prefixedDeployOutput struct {
	printer linePrinter
	prefix  string
	stage   string
}

// Spinner prints the message once: a spinner can't represent several pipelines at the same time
func (o *prefixedDeployOutput) Spinner(msg string) { o.print(o.printer.println, msg) }
func (*prefixedDeployOutput) StartSpinner()        {}
func (*prefixedDeployOutput) StopSpinner()         {}
func (o *prefixedDeployOutput) Success(msg string) { o.print(o.printer.success, msg) }
func (o *prefixedDeployOutput) Warning(msg string) { o.print(o.printer.warning, msg) }
func (o *prefixedDeployOutput) Println(line string) {
	o.print(o.printer.println, line)
}

// print prefixes every line of msg and prints them together, so lines from other pipelines can't get in between
func (o *prefixedDeployOutput) print(printFn func(stage, line string), msg string) {
	prefixedOutputMu.Lock()
	defer prefixedOutputMu.Unlock()
	for _, line := range strings.Split(strings.TrimRight(msg, "\r\n"), "\n") {
		printFn(o.stage, fmt.Sprintf("[%s] %s", o.prefix, strings.TrimRight(line, "\r")))
	}
}
