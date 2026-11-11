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
	oktetoLog "github.com/okteto/okteto/pkg/log"
)

// DeployOutput receives the messages and log lines of a pipeline deploy
type DeployOutput interface {
	Spinner(msg string)
	StartSpinner()
	StopSpinner()
	Success(msg string)
	Warning(msg string)
	Println(line string)
}

// newDeployOutput returns the output of the deploy: the one in the options, or the global logger
func newDeployOutput(opts *DeployOptions) DeployOutput {
	if opts.Output != nil {
		return opts.Output
	}
	return globalDeployOutput{}
}

// globalDeployOutput prints through the global logger, with spinner. It is the default output
type globalDeployOutput struct{}

func (globalDeployOutput) Spinner(msg string)  { oktetoLog.Spinner(msg) }
func (globalDeployOutput) StartSpinner()       { oktetoLog.StartSpinner() }
func (globalDeployOutput) StopSpinner()        { oktetoLog.StopSpinner() }
func (globalDeployOutput) Success(msg string)  { oktetoLog.Success("%s", msg) }
func (globalDeployOutput) Warning(msg string)  { oktetoLog.Warning("%s", msg) }
func (globalDeployOutput) Println(line string) { oktetoLog.Println(line) }
