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
	"github.com/okteto/okteto/pkg/types"
	"github.com/shurcooL/graphql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCatalogClient_List_Success(t *testing.T) {
	t.Helper()
	fake := &fakeGraphQLClient{
		queryResult: &getGitCatalogItemsQuery{
			Response: []gitCatalogItem{
				{
					Id:            "id-1",
					Name:          "demo",
					RepositoryUrl: "https://github.com/okteto/movies",
					Branch:        "main",
					ManifestPath:  "okteto.yml",
					ReadOnly:      false,
					Variables: []gitCatalogItemVariable{
						{Name: "TOKEN", Value: "abc"},
					},
				},
			},
		},
	}
	c := newCatalogClient(fake)

	items, err := c.List(context.Background())
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, types.GitCatalogItem{
		ID:            "id-1",
		Name:          "demo",
		RepositoryURL: "https://github.com/okteto/movies",
		Branch:        "main",
		ManifestPath:  "okteto.yml",
		ReadOnly:      false,
		Variables:     []types.GitCatalogItemVariable{{Name: "TOKEN", Value: "abc"}},
	}, items[0])
}

func TestCatalogClient_List_EmptyResponse(t *testing.T) {
	t.Helper()
	c := newCatalogClient(&fakeGraphQLClient{})

	items, err := c.List(context.Background())
	require.NoError(t, err)
	assert.Empty(t, items)
}

func TestCatalogClient_List_UnsupportedServer(t *testing.T) {
	t.Helper()
	c := newCatalogClient(&fakeGraphQLClient{
		err: errors.New(`graphql: Cannot query field "getGitCatalogItems" on type "Query"`),
	})

	_, err := c.List(context.Background())
	require.Error(t, err)
	var userErr oktetoErrors.UserError
	require.ErrorAs(t, err, &userErr)
	assert.Equal(t, ErrCatalogNotSupported, err)
}

func TestCatalogClient_List_GenericError(t *testing.T) {
	t.Helper()
	c := newCatalogClient(&fakeGraphQLClient{
		err: errors.New("boom"),
	})

	_, err := c.List(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list catalog items")
}

func TestCatalogClient_Deploy_Success(t *testing.T) {
	t.Helper()
	fake := &fakeGraphQLClient{
		mutationResult: &deployCatalogItemMutation{
			Response: deployCatalogItemResponse{
				Action: actionStruct{
					Id:     graphql.String("action-id"),
					Name:   graphql.String("cli"),
					Status: graphql.String("progressing"),
				},
				GitDeploy: gitDeployInfoWithRepoInfo{
					Id:     graphql.String("deploy-id"),
					Name:   graphql.String("demo"),
					Status: graphql.String("progressing"),
				},
			},
		},
	}
	c := newCatalogClient(fake)

	resp, err := c.Deploy(context.Background(), types.CatalogDeployOptions{
		CatalogItemID: "id-1",
		Name:          "demo",
		Repository:    "https://github.com/okteto/movies",
		Branch:        "main",
		Filename:      "okteto.yml",
		Namespace:     "ns",
		Variables:     []types.Variable{{Name: "TOKEN", Value: "abc"}},
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "deploy-id", resp.GitDeploy.ID)
	assert.Equal(t, "action-id", resp.Action.ID)
}

func TestCatalogClient_Deploy_UnsupportedServer(t *testing.T) {
	t.Helper()
	c := newCatalogClient(&fakeGraphQLClient{
		err: errors.New(`graphql: Unknown argument "catalogItemId" on field "deployGitRepository" of type "Mutation"`),
	})

	_, err := c.Deploy(context.Background(), types.CatalogDeployOptions{Name: "demo", Namespace: "ns"})
	require.Error(t, err)
	assert.Equal(t, ErrCatalogNotSupported, err)
}

func TestBuildDeployVariables_InjectsOriginWhenMissing(t *testing.T) {
	t.Helper()
	c := newCatalogClient(&fakeGraphQLClient{})

	vars := c.buildDeployVariables(types.CatalogDeployOptions{
		Name:       "demo",
		Repository: "repo",
		Namespace:  "ns",
		Variables:  []types.Variable{{Name: "FOO", Value: "bar"}},
	})

	inputs, ok := vars["variables"].([]InputVariable)
	require.True(t, ok)
	names := make([]string, 0, len(inputs))
	for _, v := range inputs {
		names = append(names, string(v.Name))
	}
	assert.Contains(t, names, "FOO")
	assert.Contains(t, names, "OKTETO_ORIGIN")
}

func TestBuildDeployVariables_PreservesUserOrigin(t *testing.T) {
	t.Helper()
	c := newCatalogClient(&fakeGraphQLClient{})

	vars := c.buildDeployVariables(types.CatalogDeployOptions{
		Name:       "demo",
		Repository: "repo",
		Namespace:  "ns",
		Variables:  []types.Variable{{Name: "OKTETO_ORIGIN", Value: "from-user"}},
	})

	inputs, ok := vars["variables"].([]InputVariable)
	require.True(t, ok)
	require.Len(t, inputs, 1)
	assert.Equal(t, "OKTETO_ORIGIN", string(inputs[0].Name))
	assert.Equal(t, "from-user", string(inputs[0].Value))
}
