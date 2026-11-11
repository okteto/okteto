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

package remoterun

import (
	"context"
	"fmt"
	"testing"

	"github.com/okteto/okteto/internal/test/client"
	"github.com/okteto/okteto/pkg/catalog"
	"github.com/okteto/okteto/pkg/deployable"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type fakeDestroyRunner struct {
	mock.Mock
}

func (f *fakeDestroyRunner) RunDestroy(ctx context.Context, params deployable.DestroyParameters) error {
	args := f.Called(params)
	return args.Error(0)
}

func TestRun_DestroyCommand(t *testing.T) {
	tests := []struct {
		name     string
		expected error
		params   deployable.DestroyParameters
	}{
		{
			name: "WithoutError",
			params: deployable.DestroyParameters{
				Name:      "test-destroy",
				Namespace: "test-namespace",
			},
			expected: nil,
		},
		{
			name: "WithError",
			params: deployable.DestroyParameters{
				Name:      "test-destroy",
				Namespace: "test-namespace",
			},
			expected: fmt.Errorf("boooooom"),
		},
	}

	okteto.CurrentStore = &okteto.ContextStore{
		CurrentContext: "test",
		Contexts: map[string]*okteto.Context{
			"test": {
				Token: "token",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeDestroyRunner{}
			runner.On("RunDestroy", tt.params).Return(tt.expected)
			command := &DestroyCommand{
				runner: runner,
			}
			err := command.Run(context.Background(), tt.params)

			require.Equal(t, err, tt.expected)
			runner.AssertExpectations(t)
		})
	}

}

func TestRunDestroysWithTheCatalogSecretVariables(t *testing.T) {
	setTestOktetoContext(t)
	runner := &fakeDestroyRunner{}
	command := &DestroyCommand{
		runner: runner,
		catalogSecretVariables: newTestCatalogSecretVariables(&client.FakeUserClient{
			CatalogSecretVariables: types.CatalogSecretVariables{
				Variables: []types.CatalogSecretVariable{{Name: "API_TOKEN", Value: "abc"}},
			},
		}, catalog.DestroyPhase),
	}
	runner.On("RunDestroy", deployable.DestroyParameters{
		Name:                   "test-destroy",
		CatalogSecretVariables: []string{"API_TOKEN=abc"},
	}).Return(nil)

	err := command.Run(context.Background(), deployable.DestroyParameters{Name: "test-destroy"})

	require.NoError(t, err)
	runner.AssertExpectations(t)
}

func TestRunDoesNotDestroyWhenCatalogSecretVariablesFail(t *testing.T) {
	setTestOktetoContext(t)
	out := captureJSONLogs(t)
	runner := &fakeDestroyRunner{}
	command := &DestroyCommand{
		runner:                 runner,
		catalogSecretVariables: newTestCatalogSecretVariables(&client.FakeUserClient{ErrCatalogSecretVariables: okteto.ErrCatalogSecretVariablesDisabled}, catalog.DestroyPhase),
	}

	err := command.Run(context.Background(), deployable.DestroyParameters{Name: "test-destroy"})

	require.ErrorIs(t, err, okteto.ErrCatalogSecretVariablesDisabled)
	runner.AssertNotCalled(t, "RunDestroy", mock.Anything)
	require.Contains(t, jsonLogs(t, out), catalogLog(oktetoLog.ErrorLevel, "secret variables are disabled on this installation"))
}

func TestRunForceDestroysWithoutTheCatalogSecretVariablesWhenTheyFail(t *testing.T) {
	setTestOktetoContext(t)
	out := captureJSONLogs(t)
	runner := &fakeDestroyRunner{}
	secretVariables := newTestCatalogSecretVariables(&client.FakeUserClient{ErrCatalogSecretVariables: okteto.ErrCatalogSecretVariablesDisabled}, catalog.DestroyPhase)
	secretVariables.forceDestroy = true
	command := &DestroyCommand{
		runner:                 runner,
		catalogSecretVariables: secretVariables,
	}
	runner.On("RunDestroy", deployable.DestroyParameters{Name: "test-destroy", ForceDestroy: true}).Return(nil)

	err := command.Run(context.Background(), deployable.DestroyParameters{Name: "test-destroy", ForceDestroy: true})

	require.NoError(t, err)
	runner.AssertExpectations(t)
	require.Equal(t, []oktetoLog.JSONLogFormat{
		catalogWarning("Running the destroy commands without the catalog secret variables: secret variables are disabled on this installation"),
	}, warnings(jsonLogs(t, out)))
	require.NotContains(t, jsonLogs(t, out), catalogLog(oktetoLog.ErrorLevel, "secret variables are disabled on this installation"))
}

func TestRunDestroysWithoutTheUnresolvedCatalogSecretVariables(t *testing.T) {
	setTestOktetoContext(t)
	out := captureJSONLogs(t)
	runner := &fakeDestroyRunner{}
	command := &DestroyCommand{
		runner:                 runner,
		catalogSecretVariables: newTestCatalogSecretVariables(withUnresolvedCatalogSecretVariables(), catalog.DestroyPhase),
	}
	runner.On("RunDestroy", deployable.DestroyParameters{
		Name:                   "test-destroy",
		CatalogSecretVariables: []string{"PIN=1"},
	}).Return(nil)

	err := command.Run(context.Background(), deployable.DestroyParameters{Name: "test-destroy"})

	require.NoError(t, err)
	runner.AssertExpectations(t)
	require.Contains(t, jsonLogs(t, out), catalogWarning(`Catalog variable "API_TOKEN" could not be resolved: Secret "creds" not found. The commands run without it`))
}

func TestDestroyCommandHasTheCatalogFlags(t *testing.T) {
	cmd := Destroy(context.Background())

	require.NotNil(t, cmd.Flags().Lookup("catalog-action"))
	require.NotNil(t, cmd.Flags().Lookup("catalog-secret-variables-endpoint"))
}

func setTestOktetoContext(t *testing.T) {
	t.Helper()
	original := okteto.CurrentStore
	t.Cleanup(func() {
		okteto.CurrentStore = original
	})
	okteto.CurrentStore = &okteto.ContextStore{
		CurrentContext: "test",
		Contexts: map[string]*okteto.Context{
			"test": {Name: "https://okteto.example", Token: "token"},
		},
	}
}
