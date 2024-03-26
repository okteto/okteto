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
	"testing"

	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDestroyExecutor struct {
	err      error
	executed []model.DeployCommand
	cleaned  bool
}

func (fe *fakeDestroyExecutor) Execute(command model.DeployCommand, _ []string) error {
	fe.executed = append(fe.executed, command)
	if fe.err != nil {
		return fe.err
	}

	return nil
}

func (fe *fakeDestroyExecutor) CleanUp(_ error) {
	fe.cleaned = true
}

func TestRunDestroyWithError(t *testing.T) {
	executor := &fakeDestroyExecutor{
		err: assert.AnError,
	}
	runner := &DestroyRunner{
		Executor: executor,
	}

	params := DestroyParameters{
		Deployable: Entity{
			Commands: []model.DeployCommand{
				{
					Name:    "cmd1",
					Command: "cmd1",
				},
				{
					Name:    "cmd2",
					Command: "cmd2",
				},
			},
		},
	}

	err := runner.RunDestroy(params)

	expectedExecutedCommands := []model.DeployCommand{
		{
			Name:    "cmd1",
			Command: "cmd1",
		},
	}
	require.Error(t, err)
	require.ElementsMatch(t, expectedExecutedCommands, executor.executed)
}

func TestRunDestroyWithErrorAndForceDestroy(t *testing.T) {
	executor := &fakeDestroyExecutor{
		err: assert.AnError,
	}
	runner := &DestroyRunner{
		Executor: executor,
	}

	params := DestroyParameters{
		Deployable: Entity{
			Commands: []model.DeployCommand{
				{
					Name:    "cmd1",
					Command: "cmd1",
				},
				{
					Name:    "cmd2",
					Command: "cmd2",
				},
			},
		},
		ForceDestroy: true,
	}

	err := runner.RunDestroy(params)

	expectedExecutedCommands := []model.DeployCommand{
		{
			Name:    "cmd1",
			Command: "cmd1",
		},
		{
			Name:    "cmd2",
			Command: "cmd2",
		},
	}
	require.Error(t, err)
	require.ElementsMatch(t, expectedExecutedCommands, executor.executed)
}

func TestRunDestroyWithoutError(t *testing.T) {
	executor := &fakeDestroyExecutor{}
	runner := &DestroyRunner{
		Executor: executor,
	}

	params := DestroyParameters{
		Deployable: Entity{
			Commands: []model.DeployCommand{
				{
					Name:    "cmd1",
					Command: "cmd1",
				},
				{
					Name:    "cmd2",
					Command: "cmd2",
				},
			},
		},
		ForceDestroy: true,
	}

	err := runner.RunDestroy(params)

	expectedExecutedCommands := []model.DeployCommand{
		{
			Name:    "cmd1",
			Command: "cmd1",
		},
		{
			Name:    "cmd2",
			Command: "cmd2",
		},
	}
	require.NoError(t, err)
	require.ElementsMatch(t, expectedExecutedCommands, executor.executed)
}

func TestCleanUp(t *testing.T) {
	executor := &fakeDestroyExecutor{}
	runner := &DestroyRunner{
		Executor: executor,
	}

	runner.CleanUp(nil)

	require.True(t, executor.cleaned)
}
