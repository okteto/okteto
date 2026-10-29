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
	"bytes"
	"context"
	"os"
	"testing"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// printingExecutor prints its output through the okteto logger, as the real
// executor does with the output of the commands
type printingExecutor struct {
	output string
}

func (e printingExecutor) Execute(_ model.DeployCommand, _ []string) error {
	oktetoLog.Println(e.output)
	return nil
}

func (printingExecutor) CleanUp(_ error) {}

// captureLogOutput sends the okteto logger output to the returned buffer, with
// masking disabled, until the test finishes
func captureLogOutput(t *testing.T) *bytes.Buffer {
	t.Helper()
	out := &bytes.Buffer{}
	oktetoLog.Init(logrus.InfoLevel)
	oktetoLog.DisableMasking()
	oktetoLog.SetOutputFormat(oktetoLog.PlainFormat)
	oktetoLog.SetOutput(out)
	t.Cleanup(func() {
		oktetoLog.DisableMasking()
		oktetoLog.Init(logrus.InfoLevel)
	})
	return out
}

func TestRunDestroyMasksCommandOutput(t *testing.T) {
	setFakeOktetoContext(t)
	out := captureLogOutput(t)
	// Added before the runner enables masking, like a destroy that already
	// enabled it before the platform values were added
	oktetoLog.AddMaskedWord("platform-secret")

	runner := &DestroyRunner{
		Executor: printingExecutor{output: "token: platform-secret"},
	}
	err := runner.RunDestroy(context.Background(), DestroyParameters{
		Deployable: Entity{
			Commands: []model.DeployCommand{{Name: "print", Command: "print"}},
		},
	})

	require.NoError(t, err)
	assert.Contains(t, out.String(), "token: ***")
	assert.NotContains(t, out.String(), "platform-secret")
}

type fakeDestroyExecutor struct {
	err      error
	executed []model.DeployCommand
	envs     [][]string
	cleaned  bool
}

func (fe *fakeDestroyExecutor) Execute(command model.DeployCommand, env []string) error {
	fe.executed = append(fe.executed, command)
	fe.envs = append(fe.envs, env)
	if fe.err != nil {
		return fe.err
	}

	return nil
}

func (fe *fakeDestroyExecutor) CleanUp(_ error) {
	fe.cleaned = true
}

func TestRunDestroyWithError(t *testing.T) {
	setFakeOktetoContext(t)

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

	err := runner.RunDestroy(context.Background(), params)

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
	setFakeOktetoContext(t)

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

	err := runner.RunDestroy(context.Background(), params)

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
	originalStore := okteto.CurrentStore
	defer func() { okteto.CurrentStore = originalStore }()

	okteto.CurrentStore = &okteto.ContextStore{
		CurrentContext: "test",
		Contexts: map[string]*okteto.Context{
			"test": {
				Gateway: &okteto.GatewayMetadata{
					Name:      "dev-gateway",
					Namespace: "gateway-ns",
				},
			},
		},
	}

	executor := &fakeDestroyExecutor{}
	runner := &DestroyRunner{
		Executor: executor,
	}

	params := DestroyParameters{
		Variables: []string{"A=value1"},
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

	err := runner.RunDestroy(context.Background(), params)

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
	require.Len(t, executor.envs, 2)
	require.Equal(t, []string{
		"A=value1",
		"OKTETO_DEV_GATEWAY_NAME=dev-gateway",
		"OKTETO_DEV_GATEWAY_NAMESPACE=gateway-ns",
	}, executor.envs[0])
}

func TestCleanUp(t *testing.T) {
	executor := &fakeDestroyExecutor{}
	runner := &DestroyRunner{
		Executor: executor,
	}

	runner.CleanUp(nil)

	require.True(t, executor.cleaned)
}

func TestRunDestroyAddsCatalogSecretVariablesAfterTheVariables(t *testing.T) {
	setFakeOktetoContext(t)
	exec := &fakeDestroyExecutor{}
	runner := &DestroyRunner{Executor: exec}

	err := runner.RunDestroy(context.Background(), DestroyParameters{
		Variables:              []string{"CATALOG_TEST_TOKEN=from-var"},
		CatalogSecretVariables: []string{"CATALOG_TEST_TOKEN=secret-value"},
		Deployable: Entity{
			Commands: []model.DeployCommand{{Name: "destroy", Command: "destroy"}},
		},
	})

	require.NoError(t, err)
	assert.Equal(t, [][]string{{"CATALOG_TEST_TOKEN=from-var", "CATALOG_TEST_TOKEN=secret-value"}}, exec.envs)
	assert.Empty(t, os.Getenv("CATALOG_TEST_TOKEN"))
}

func TestRunDestroyMasksShortCatalogSecretValues(t *testing.T) {
	setFakeOktetoContext(t)
	out := captureLogOutput(t)
	runner := &DestroyRunner{
		Executor: printingExecutor{output: "pin: 1234"},
	}

	err := runner.RunDestroy(context.Background(), DestroyParameters{
		CatalogSecretVariables: []string{"PIN=1234"},
		Deployable: Entity{
			Commands: []model.DeployCommand{{Name: "print", Command: "print"}},
		},
	})

	require.NoError(t, err)
	assert.Contains(t, out.String(), "pin: ***")
	assert.NotContains(t, out.String(), "1234")
}
