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

package catalog

import (
	"testing"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/require"
)

var testSecretVariables = SecretVariables{
	References: `[{"name":"API_TOKEN","secretKeyRef":{"name":"creds","key":"token"}}]`,
	Action:     "action-name",
	Endpoint:   "https://okteto.example/internal/catalog/secret-variables",
}

// remote reports whether runsInRemote was called, and returns inRemote
type remote struct {
	inRemote bool
	called   bool
}

func (r *remote) runsInRemote() bool {
	r.called = true
	return r.inRemote
}

func TestSecretVariablesFromEnv(t *testing.T) {
	t.Setenv(SecretVariablesEnvVar, testSecretVariables.References)
	t.Setenv(model.OktetoActionNameEnvVar, testSecretVariables.Action)
	t.Setenv(SecretVariablesEndpointEnvVar, testSecretVariables.Endpoint)

	require.Equal(t, testSecretVariables, SecretVariablesFromEnv())
}

func TestSecretVariablesFromEnvWithoutVariables(t *testing.T) {
	t.Setenv(SecretVariablesEnvVar, "")
	t.Setenv(model.OktetoActionNameEnvVar, "")
	t.Setenv(SecretVariablesEndpointEnvVar, "")

	require.Equal(t, SecretVariables{}, SecretVariablesFromEnv())
}

func TestRemoteRunFlags(t *testing.T) {
	expected := []string{
		`--catalog-action "action-name"`,
		`--catalog-secret-variables-endpoint "https://okteto.example/internal/catalog/secret-variables"`,
	}

	require.Equal(t, expected, testSecretVariables.RemoteRunFlags())
}

func TestRemoteRunFlagsWithoutAction(t *testing.T) {
	require.Empty(t, SecretVariables{}.RemoteRunFlags())
}

func TestCheckSecretVariablesWithoutReferences(t *testing.T) {
	r := &remote{}

	got, err := CheckSecretVariables(SecretVariables{Action: "action-name"}, DeployPhase, r.runsInRemote)

	require.NoError(t, err)
	require.Equal(t, SecretVariables{}, got)
	require.False(t, r.called)
}

func TestCheckSecretVariablesWithAnEmptyList(t *testing.T) {
	r := &remote{}
	s := testSecretVariables
	s.References = "[]"

	got, err := CheckSecretVariables(s, DeployPhase, r.runsInRemote)

	require.NoError(t, err)
	require.Equal(t, SecretVariables{}, got)
	require.False(t, r.called)
}

func TestCheckSecretVariablesInRemote(t *testing.T) {
	phases := []Phase{DeployPhase, DestroyPhase}
	for _, phase := range phases {
		t.Run(string(phase), func(t *testing.T) {
			got, err := CheckSecretVariables(testSecretVariables, phase, (&remote{inRemote: true}).runsInRemote)

			require.NoError(t, err)
			require.Equal(t, testSecretVariables, got)
		})
	}
}

func TestCheckSecretVariablesFailsOnLocalDeploy(t *testing.T) {
	got, err := CheckSecretVariables(testSecretVariables, DeployPhase, (&remote{}).runsInRemote)

	require.ErrorIs(t, err, ErrSecretVariablesNeedRemote)
	require.EqualError(t, err, "catalog variables of type `secret` are only supported with deploy on remote. Enable remote execution and try again")
	require.Equal(t, SecretVariables{}, got)
	var uErr oktetoErrors.UserError
	require.ErrorAs(t, err, &uErr)
	require.Empty(t, uErr.Hint)
}

func TestCheckSecretVariablesFailsOnLocalDestroyPointingToForceDestroy(t *testing.T) {
	_, err := CheckSecretVariables(testSecretVariables, DestroyPhase, (&remote{}).runsInRemote)

	require.ErrorIs(t, err, ErrSecretVariablesNeedRemote)
	var uErr oktetoErrors.UserError
	require.ErrorAs(t, err, &uErr)
	require.Equal(t, forceDestroyHint, uErr.Hint)
}

func TestCheckSecretVariablesFailsWithoutEndpoint(t *testing.T) {
	s := testSecretVariables
	s.Endpoint = ""

	got, err := CheckSecretVariables(s, DeployPhase, (&remote{inRemote: true}).runsInRemote)

	require.ErrorIs(t, err, errSecretVariablesEndpointNotSet)
	require.Equal(t, SecretVariables{}, got)
}

func TestCheckSecretVariablesFailsWithoutAction(t *testing.T) {
	s := testSecretVariables
	s.Action = ""

	got, err := CheckSecretVariables(s, DeployPhase, (&remote{inRemote: true}).runsInRemote)

	require.ErrorIs(t, err, errActionNotSet)
	require.EqualError(t, err, "the development environment has catalog secret variables, but OKTETO_ACTION_NAME is not set")
	require.Equal(t, SecretVariables{}, got)
}

func TestCheckSecretVariablesFailsWithInvalidReferences(t *testing.T) {
	phases := []Phase{DeployPhase, DestroyPhase}
	for _, phase := range phases {
		t.Run(string(phase), func(t *testing.T) {
			r := &remote{inRemote: true}
			s := testSecretVariables
			s.References = `{"name":`

			got, err := CheckSecretVariables(s, phase, r.runsInRemote)

			require.ErrorContains(t, err, "could not read the catalog secret variables from OKTETO_CATALOG_SECRET_VARIABLES")
			require.Equal(t, SecretVariables{}, got)
			require.False(t, r.called)
		})
	}
}
