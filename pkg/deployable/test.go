// Copyright 2024 The Okteto Authors
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

package deployable

import (
	"fmt"

	"github.com/okteto/okteto/cmd/utils/executor"
	oktetoLog "github.com/okteto/okteto/pkg/log"
)

// TestRunner is responsible for running the commands defined in a manifest when
// running tests
type TestRunner struct {
	Executor executor.ManifestExecutor
}

// TestParameters represents the parameters for destroying a remote entity
type TestParameters struct {
	Name         string
	Namespace    string
	Deployable   Entity
	Variables    []string
	ForceDestroy bool
}

// RunTest executes the custom commands received as part of TestParameters
func (dr *TestRunner) RunTest(params TestParameters) error {
	var commandErr error
	lastCommandName := ""
	for _, command := range params.Deployable.Commands {
		oktetoLog.Information("Running '%s'", command.Name)
		lastCommandName = command.Name
		oktetoLog.SetStage(command.Name)
		if err := dr.Executor.Execute(command, params.Variables); err != nil {
			err = fmt.Errorf("error executing command '%s': %w", command.Name, err)

			// Store the error to return if the force destroy option is set
			commandErr = err
		}
		oktetoLog.SetStage("")
	}

	// This is a hack for improving the logs until we refactor all that. The oktetoLog.Information('Running '%s'')
	// should not appear under any stage, that is why we clear the stage after each execution. To keep backward compatibility
	// in case of failure of command, we end up the function setting the stage to the last command executed.
	oktetoLog.SetStage(lastCommandName)

	return commandErr
}
