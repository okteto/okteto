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

package okteto

import (
	"context"
	"errors"
	"testing"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCatalogItemSecretVariables(t *testing.T) {
	pc := &pipelineClient{
		client: &fakeGraphQLClient{
			queryResult: &getCatalogItemSecretVariablesQuery{
				Response: catalogItemSecretVariables{
					SecretVariables: []catalogItemSecretVariableName{{Name: "API_TOKEN"}, {Name: "DB_PASSWORD"}},
				},
			},
		},
	}

	names, err := pc.GetCatalogItemSecretVariables(context.Background(), "item-id")

	require.NoError(t, err)
	require.Equal(t, []string{"API_TOKEN", "DB_PASSWORD"}, names)
}

func TestGetCatalogItemSecretVariablesWithoutSecretVariables(t *testing.T) {
	pc := &pipelineClient{
		client: &fakeGraphQLClient{
			queryResult: &getCatalogItemSecretVariablesQuery{},
		},
	}

	names, err := pc.GetCatalogItemSecretVariables(context.Background(), "item-id")

	require.NoError(t, err)
	require.Empty(t, names)
}

func TestGetCatalogItemSecretVariablesNotFound(t *testing.T) {
	pc := &pipelineClient{
		client: &fakeGraphQLClient{
			err: errors.New("graphql: git-catalog-item-not-found"),
		},
	}

	_, err := pc.GetCatalogItemSecretVariables(context.Background(), "item-id")

	require.ErrorIs(t, err, oktetoErrors.ErrCatalogItemNotFound)
}

func TestGetCatalogItemSecretVariablesError(t *testing.T) {
	pc := &pipelineClient{
		client: &fakeGraphQLClient{
			err: assert.AnError,
		},
	}

	_, err := pc.GetCatalogItemSecretVariables(context.Background(), "item-id")

	require.ErrorIs(t, err, assert.AnError)
}
