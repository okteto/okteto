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
			CatalogSecretVariables: []types.CatalogSecretVariable{{Name: "API_TOKEN", Value: "abc"}},
		}, destroyRetryHint),
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
		catalogSecretVariables: newTestCatalogSecretVariables(&client.FakeUserClient{ErrCatalogSecretVariables: unresolvedCatalogSecretVariables}, destroyRetryHint),
	}

	err := command.Run(context.Background(), deployable.DestroyParameters{Name: "test-destroy", ForceDestroy: true})

	require.ErrorAs(t, err, &okteto.UnresolvedCatalogSecretVariablesError{})
	runner.AssertNotCalled(t, "RunDestroy", mock.Anything)
	require.Contains(t, jsonLogs(t, out), catalogLog(oktetoLog.ErrorLevel, "the secret variables of the catalog item could not be resolved"))
}

func TestDestroyCommandHasTheCatalogFlags(t *testing.T) {
	cmd := Destroy(context.Background())

	require.NotNil(t, cmd.Flags().Lookup("catalog-item-id"))
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
			"test": {Token: "token"},
		},
	}
}
