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

// DestroyRunner is responsible for running the commands defined in a manifest when destroying
// a dev environment
// This DestroyRunner has the common functionality to deal with the custom commands when destroy is
// run locally or remotely. As this runs also in the remote, it should NEVER build any kind of image
// or execute some logic that might differ from local.
type DestroyRunner struct {
	Executor executor.ManifestExecutor
}

// DestroyParameters represents the parameters for destroying a remote entity
type DestroyParameters struct {
	Name         string
	Namespace    string
	Deployable   Entity
	Variables    []string
	ForceDestroy bool
}

// RunDestroy executes the custom commands received as part of DestroyParameters
func (dr *DestroyRunner) RunDestroy(params DestroyParameters) error {
	var commandErr error
	lastCommandName := ""
	for _, command := range params.Deployable.Commands {
		oktetoLog.Information("Running '%s'", command.Name)
		lastCommandName = command.Name
		oktetoLog.SetStage(command.Name)
		if err := dr.Executor.Execute(command, params.Variables); err != nil {
			err = fmt.Errorf("error executing command '%s': %w", command.Name, err)
			// In case of force destroy, we have to execute all commands even if a single one fails
			if !params.ForceDestroy {
				return err
			}

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

func (dr *DestroyRunner) CleanUp(err error) {
	if dr.Executor != nil {
		dr.Executor.CleanUp(err)
	}
	oktetoLog.Debugf("executed clean up completely")
}
