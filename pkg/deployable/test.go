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
	"github.com/okteto/okteto/cmd/utils/executor"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/spf13/afero"
)

// TestRunner is responsible for running the commands defined in a manifest when
// running tests
type TestRunner struct {
	Executor executor.ManifestExecutor
	Fs       afero.Fs
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
	oktetoLog.SetStage(params.Name)

	oktetoEnvFile, unlinkEnv, err := createTempOktetoEnvFile(dr.Fs)
	if err != nil {
		return err
	}

	envStepper := NewEnvStepper(oktetoEnvFile.Name())
	envStepper.WithFS(dr.Fs)

	defer unlinkEnv()

	for _, command := range params.Deployable.Commands {
		oktetoLog.Information("Running '%s'", command.Name)

		execEnv := []string{}
		execEnv = append(execEnv, params.Variables...)

		if err := dr.Executor.Execute(command, execEnv); err != nil {
			return err
		}

		// Read variables that may have been written to OKTETO_ENV in the current step
		envsFromOktetoEnvFile, err := envStepper.Step()
		if err != nil {
			oktetoLog.Warning("no valid format used in the okteto env file: %s", err.Error())
		}

		params.Variables = append(params.Variables, envsFromOktetoEnvFile...)
	}

	return nil
}
