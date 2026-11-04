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
	"context"
	"testing"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testItem = Item{
	ID:                      "item-id",
	SecretVariablesEndpoint: "https://okteto.example/internal/catalog/secret-variables",
}

type fakeSecretVariables struct {
	err   error
	names []string
	ids   []string
}

func (f *fakeSecretVariables) get(_ context.Context, id string) ([]string, error) {
	f.ids = append(f.ids, id)
	return f.names, f.err
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

func TestItemFromEnv(t *testing.T) {
	t.Setenv(ItemIDEnvVar, testItem.ID)
	t.Setenv(SecretVariablesEndpointEnvVar, testItem.SecretVariablesEndpoint)

	require.Equal(t, testItem, ItemFromEnv())
}

func TestItemFromEnvWithoutVariables(t *testing.T) {
	t.Setenv(ItemIDEnvVar, "")
	t.Setenv(SecretVariablesEndpointEnvVar, "")

	require.Equal(t, Item{}, ItemFromEnv())
}

func TestRemoteRunFlags(t *testing.T) {
	expected := []string{
		`--catalog-item-id "item-id"`,
		`--catalog-secret-variables-endpoint "https://okteto.example/internal/catalog/secret-variables"`,
	}

	require.Equal(t, expected, testItem.RemoteRunFlags())
}

func TestRemoteRunFlagsWithoutItem(t *testing.T) {
	require.Empty(t, Item{}.RemoteRunFlags())
}

func TestCheckSecretVariablesWithoutItem(t *testing.T) {
	secrets := &fakeSecretVariables{names: []string{"API_TOKEN"}}
	r := &remote{}

	item, err := CheckSecretVariables(context.Background(), Item{}, DeployPhase, secrets.get, r.runsInRemote)

	require.NoError(t, err)
	require.Equal(t, Item{}, item)
	require.Empty(t, secrets.ids)
	require.False(t, r.called)
}

func TestCheckSecretVariablesWithoutSecretVariables(t *testing.T) {
	secrets := &fakeSecretVariables{}
	r := &remote{}

	item, err := CheckSecretVariables(context.Background(), testItem, DeployPhase, secrets.get, r.runsInRemote)

	require.NoError(t, err)
	require.Equal(t, Item{}, item)
	require.Equal(t, []string{"item-id"}, secrets.ids)
	require.False(t, r.called)
}

func TestCheckSecretVariablesInRemote(t *testing.T) {
	phases := []Phase{DeployPhase, DestroyPhase}
	for _, phase := range phases {
		t.Run(string(phase), func(t *testing.T) {
			secrets := &fakeSecretVariables{names: []string{"API_TOKEN"}}

			item, err := CheckSecretVariables(context.Background(), testItem, phase, secrets.get, (&remote{inRemote: true}).runsInRemote)

			require.NoError(t, err)
			require.Equal(t, testItem, item)
		})
	}
}

func TestCheckSecretVariablesFailsOnLocalDeploy(t *testing.T) {
	secrets := &fakeSecretVariables{names: []string{"API_TOKEN"}}

	item, err := CheckSecretVariables(context.Background(), testItem, DeployPhase, secrets.get, (&remote{}).runsInRemote)

	require.ErrorIs(t, err, ErrSecretVariablesNeedRemote)
	require.EqualError(t, err, "catalog variables of type `secret` are only supported with deploy on remote. Enable remote execution and try again")
	require.Equal(t, Item{}, item)
	var uErr oktetoErrors.UserError
	require.ErrorAs(t, err, &uErr)
	require.Empty(t, uErr.Hint)
}

func TestCheckSecretVariablesFailsOnLocalDestroyPointingToForceDestroy(t *testing.T) {
	secrets := &fakeSecretVariables{names: []string{"API_TOKEN"}}

	_, err := CheckSecretVariables(context.Background(), testItem, DestroyPhase, secrets.get, (&remote{}).runsInRemote)

	require.ErrorIs(t, err, ErrSecretVariablesNeedRemote)
	var uErr oktetoErrors.UserError
	require.ErrorAs(t, err, &uErr)
	require.Equal(t, forceDestroyHint, uErr.Hint)
}

func TestCheckSecretVariablesFailsWithoutEndpoint(t *testing.T) {
	secrets := &fakeSecretVariables{names: []string{"API_TOKEN"}}

	item, err := CheckSecretVariables(context.Background(), Item{ID: "item-id"}, DeployPhase, secrets.get, (&remote{inRemote: true}).runsInRemote)

	require.ErrorIs(t, err, errSecretVariablesEndpointNotSet)
	require.Equal(t, Item{}, item)
}

func TestCheckSecretVariablesFailsWhenTheyCannotBeChecked(t *testing.T) {
	phases := []Phase{DeployPhase, DestroyPhase}
	for _, phase := range phases {
		t.Run(string(phase), func(t *testing.T) {
			secrets := &fakeSecretVariables{err: assert.AnError}
			r := &remote{inRemote: true}

			item, err := CheckSecretVariables(context.Background(), testItem, phase, secrets.get, r.runsInRemote)

			require.ErrorIs(t, err, assert.AnError)
			require.ErrorContains(t, err, "could not check the secret variables of catalog item 'item-id'")
			require.Equal(t, Item{}, item)
			require.False(t, r.called)
		})
	}
}

func TestCheckSecretVariablesDeploysADeletedItemWithoutThem(t *testing.T) {
	secrets := &fakeSecretVariables{err: oktetoErrors.ErrCatalogItemNotFound}
	r := &remote{}

	item, err := CheckSecretVariables(context.Background(), testItem, DeployPhase, secrets.get, r.runsInRemote)

	require.NoError(t, err)
	require.Equal(t, Item{}, item)
	require.False(t, r.called)
}

func TestCheckSecretVariablesFailsToDestroyADeletedItem(t *testing.T) {
	secrets := &fakeSecretVariables{err: oktetoErrors.ErrCatalogItemNotFound}

	item, err := CheckSecretVariables(context.Background(), testItem, DestroyPhase, secrets.get, (&remote{inRemote: true}).runsInRemote)

	require.EqualError(t, err, "catalog item 'item-id' not found: can't tell whether its destroy commands need secret variables")
	require.Equal(t, Item{}, item)
	var uErr oktetoErrors.UserError
	require.ErrorAs(t, err, &uErr)
	require.Equal(t, forceDestroyHint, uErr.Hint)
}
