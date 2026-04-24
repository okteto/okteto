// Copyright 2025 The Okteto Authors
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
	"errors"
	"testing"
	"time"

	"github.com/okteto/okteto/internal/test/client"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVariableOverrides(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		want    []types.Variable
		wantErr bool
	}{
		{
			name:  "empty slice",
			input: nil,
			want:  []types.Variable{},
		},
		{
			name:  "single valid pair",
			input: []string{"FOO=bar"},
			want:  []types.Variable{{Name: "FOO", Value: "bar"}},
		},
		{
			name:  "value containing equals preserved",
			input: []string{"TOKEN=abc=def"},
			want:  []types.Variable{{Name: "TOKEN", Value: "abc=def"}},
		},
		{
			name:  "empty value allowed",
			input: []string{"EMPTY="},
			want:  []types.Variable{{Name: "EMPTY", Value: ""}},
		},
		{
			name:    "missing equals rejected",
			input:   []string{"NOTPAIR"},
			wantErr: true,
		},
		{
			name:    "empty key rejected",
			input:   []string{"=value"},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseVariableOverrides(tc.input)
			assert.Equal(t, tc.wantErr, err != nil)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestMergeVariables_OverridesReplaceDefaults(t *testing.T) {
	defaults := []types.GitCatalogItemVariable{
		{Name: "FOO", Value: "default-foo"},
		{Name: "BAR", Value: "default-bar"},
	}
	overrides := []types.Variable{
		{Name: "FOO", Value: "user-foo"},
		{Name: "BAZ", Value: "user-baz"},
	}
	merged := mergeVariables(defaults, overrides)
	require.Len(t, merged, 3)
	byName := map[string]string{}
	for _, v := range merged {
		byName[v.Name] = v.Value
	}
	assert.Equal(t, "user-foo", byName["FOO"])
	assert.Equal(t, "default-bar", byName["BAR"])
	assert.Equal(t, "user-baz", byName["BAZ"])
}

func TestMergeVariables_PreservesDefaultOrder(t *testing.T) {
	defaults := []types.GitCatalogItemVariable{
		{Name: "A"}, {Name: "B"}, {Name: "C"},
	}
	merged := mergeVariables(defaults, nil)
	require.Len(t, merged, 3)
	assert.Equal(t, "A", merged[0].Name)
	assert.Equal(t, "B", merged[1].Name)
	assert.Equal(t, "C", merged[2].Name)
}

func TestMergeVariables_EmptyInputs(t *testing.T) {
	merged := mergeVariables(nil, nil)
	assert.Empty(t, merged)
}

func TestMergeVariables_RepeatedOverrideKeyKeepsLast(t *testing.T) {
	// Documents `--var FOO=a --var FOO=b` semantics: the last override wins
	// and the variable is only emitted once.
	overrides := []types.Variable{
		{Name: "FOO", Value: "first"},
		{Name: "FOO", Value: "second"},
	}
	merged := mergeVariables(nil, overrides)
	require.Len(t, merged, 1)
	assert.Equal(t, "FOO", merged[0].Name)
	assert.Equal(t, "second", merged[0].Value)
}

func TestFindCatalogItem(t *testing.T) {
	items := []types.GitCatalogItem{
		{ID: "1", Name: "alpha"},
		{ID: "2", Name: "beta"},
	}
	tests := []struct {
		name     string
		lookup   string
		wantID   string
		wantFind bool
	}{
		{name: "exact match", lookup: "alpha", wantID: "1", wantFind: true},
		{name: "second entry", lookup: "beta", wantID: "2", wantFind: true},
		{name: "no match", lookup: "gamma", wantFind: false},
		{name: "empty name", lookup: "", wantFind: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := findCatalogItem(items, tc.lookup)
			assert.Equal(t, tc.wantFind, ok)
			assert.Equal(t, tc.wantID, got.ID)
		})
	}
}

func TestFirstNonEmpty(t *testing.T) {
	tests := []struct {
		name   string
		inputs []string
		want   string
	}{
		{name: "first wins", inputs: []string{"a", "b"}, want: "a"},
		{name: "skips empty", inputs: []string{"", "b"}, want: "b"},
		{name: "all empty", inputs: []string{"", ""}, want: ""},
		{name: "no args", inputs: nil, want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, firstNonEmpty(tc.inputs...))
		})
	}
}

func TestClosestName_SuggestsCloseMatch(t *testing.T) {
	items := []types.GitCatalogItem{
		{Name: "demo-app"},
		{Name: "movies"},
		{Name: "api"},
	}
	assert.Equal(t, "demo-app", closestName("dem-app", items))
}

func TestClosestName_CaseInsensitive(t *testing.T) {
	items := []types.GitCatalogItem{{Name: "MyApp"}}
	assert.Equal(t, "MyApp", closestName("myapp", items))
}

func TestClosestName_NoSuggestionWhenTooFar(t *testing.T) {
	items := []types.GitCatalogItem{{Name: "demo-app"}}
	assert.Empty(t, closestName("totally-unrelated-name", items))
}

func TestClosestName_EmptyItems(t *testing.T) {
	assert.Empty(t, closestName("anything", nil))
}

func TestNotFoundError_IncludesSuggestionWhenClose(t *testing.T) {
	items := []types.GitCatalogItem{{Name: "demo-app"}}
	err := notFoundError("demoapp", items)
	require.Error(t, err)
	assert.ErrorIs(t, err, errCatalogItemNotFound)
	assert.Contains(t, err.Error(), `Did you mean "demo-app"?`)
}

func TestNotFoundError_OmitsSuggestionWhenFar(t *testing.T) {
	items := []types.GitCatalogItem{{Name: "demo-app"}}
	err := notFoundError("not-a-match-at-all", items)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "Did you mean")
}

// deployTestFixture assembles a Command backed by fake clients so ExecuteDeploy
// can be tested without hitting the network.
type deployTestFixture struct {
	catalogResp  *client.FakeCatalogResponse
	pipelineResp *client.FakePipelineResponses
	streamResp   *client.FakeStreamResponse
	cmd          *Command
}

func newDeployTestFixture(t *testing.T, items []types.GitCatalogItem) *deployTestFixture {
	t.Helper()
	okteto.CurrentStore = &okteto.ContextStore{
		CurrentContext: "test",
		Contexts: map[string]*okteto.Context{
			"test": {Name: "test", Namespace: "my-ns"},
		},
	}
	catalogResp := &client.FakeCatalogResponse{
		CatalogItems: items,
		DeployResp: &types.GitDeployResponse{
			Action:    &types.Action{ID: "action-id", Name: "cli", Status: "progressing"},
			GitDeploy: &types.GitDeploy{ID: "deploy-id", Name: "demo", Status: "progressing"},
		},
	}
	pipelineResp := &client.FakePipelineResponses{}
	streamResp := &client.FakeStreamResponse{}
	return &deployTestFixture{
		catalogResp:  catalogResp,
		pipelineResp: pipelineResp,
		streamResp:   streamResp,
		cmd: &Command{
			okClient: &client.FakeOktetoClient{
				CatalogClient:  client.NewFakeCatalogClient(catalogResp),
				PipelineClient: client.NewFakePipelineClient(pipelineResp),
				StreamClient:   client.NewFakeStreamClient(streamResp),
			},
		},
	}
}

func TestExecuteDeploy_AppliesCatalogDefaultsAndOverrides(t *testing.T) {
	items := []types.GitCatalogItem{
		{
			ID:            "item-1",
			Name:          "demo",
			RepositoryURL: "https://github.com/okteto/movies",
			Branch:        "main",
			ManifestPath:  "okteto.yml",
			Variables: []types.GitCatalogItemVariable{
				{Name: "API_TOKEN", Value: "default-token"},
				{Name: "REGION", Value: "eu-west-1"},
			},
		},
	}
	fx := newDeployTestFixture(t, items)

	flags := &deployFlags{
		variables: []string{"API_TOKEN=override"},
		timeout:   time.Second,
		wait:      false,
	}
	err := fx.cmd.ExecuteDeploy(context.Background(), "demo", flags)
	require.NoError(t, err)

	opts := fx.catalogResp.LastDeployOpts
	assert.Equal(t, "item-1", opts.CatalogItemID)
	assert.Equal(t, "demo", opts.Name)
	assert.Equal(t, "https://github.com/okteto/movies", opts.Repository)
	assert.Equal(t, "main", opts.Branch)
	assert.Equal(t, "okteto.yml", opts.Filename)
	assert.Equal(t, "my-ns", opts.Namespace)

	byName := map[string]string{}
	for _, v := range opts.Variables {
		byName[v.Name] = v.Value
	}
	assert.Equal(t, "override", byName["API_TOKEN"])
	assert.Equal(t, "eu-west-1", byName["REGION"])
}

func TestExecuteDeploy_FlagOverridesWinOverCatalogFields(t *testing.T) {
	items := []types.GitCatalogItem{
		{ID: "item-1", Name: "demo", RepositoryURL: "repo", Branch: "main", ManifestPath: "okteto.yml"},
	}
	fx := newDeployTestFixture(t, items)

	flags := &deployFlags{
		name:      "custom-name",
		namespace: "other-ns",
		branch:    "feature/x",
		file:      "custom/okteto.yml",
		wait:      false,
	}
	require.NoError(t, fx.cmd.ExecuteDeploy(context.Background(), "demo", flags))

	opts := fx.catalogResp.LastDeployOpts
	assert.Equal(t, "custom-name", opts.Name)
	assert.Equal(t, "other-ns", opts.Namespace)
	assert.Equal(t, "feature/x", opts.Branch)
	assert.Equal(t, "custom/okteto.yml", opts.Filename)
}

func TestExecuteDeploy_NotFoundSuggestsSimilar(t *testing.T) {
	items := []types.GitCatalogItem{{ID: "item-1", Name: "demo-app"}}
	fx := newDeployTestFixture(t, items)

	err := fx.cmd.ExecuteDeploy(context.Background(), "demoapp", &deployFlags{wait: false})
	require.Error(t, err)
	assert.ErrorIs(t, err, errCatalogItemNotFound)
	assert.Contains(t, err.Error(), `Did you mean "demo-app"?`)
}

func TestExecuteDeploy_PropagatesListError(t *testing.T) {
	fx := newDeployTestFixture(t, nil)
	fx.catalogResp.ErrList = errors.New("boom")

	err := fx.cmd.ExecuteDeploy(context.Background(), "demo", &deployFlags{wait: false})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

func TestExecuteDeploy_PropagatesDeployError(t *testing.T) {
	items := []types.GitCatalogItem{{ID: "item-1", Name: "demo", RepositoryURL: "repo"}}
	fx := newDeployTestFixture(t, items)
	fx.catalogResp.ErrDeploy = errors.New("deploy boom")

	err := fx.cmd.ExecuteDeploy(context.Background(), "demo", &deployFlags{wait: false})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deploy boom")
}

func TestExecuteDeploy_WaitPathSucceeds(t *testing.T) {
	items := []types.GitCatalogItem{{ID: "item-1", Name: "demo", RepositoryURL: "repo"}}
	fx := newDeployTestFixture(t, items)

	err := fx.cmd.ExecuteDeploy(context.Background(), "demo", &deployFlags{wait: true, timeout: time.Second})
	require.NoError(t, err)
}

func TestExecuteDeploy_WaitPathSurfacesActionError(t *testing.T) {
	items := []types.GitCatalogItem{{ID: "item-1", Name: "demo", RepositoryURL: "repo"}}
	fx := newDeployTestFixture(t, items)
	fx.pipelineResp.WaitErr = errors.New("action failed")

	err := fx.cmd.ExecuteDeploy(context.Background(), "demo", &deployFlags{wait: true, timeout: time.Second})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "action failed")
}

func TestExecuteDeploy_PromptsWhenNameOmitted(t *testing.T) {
	items := []types.GitCatalogItem{
		{ID: "item-1", Name: "alpha", RepositoryURL: "repo-a"},
		{ID: "item-2", Name: "beta", RepositoryURL: "repo-b"},
	}
	fx := newDeployTestFixture(t, items)

	var received []types.GitCatalogItem
	picker := func(list []types.GitCatalogItem) (string, error) {
		received = list
		return "beta", nil
	}
	err := fx.cmd.executeDeploy(context.Background(), "", &deployFlags{wait: false}, picker)
	require.NoError(t, err)

	assert.Len(t, received, 2)
	assert.Equal(t, "item-2", fx.catalogResp.LastDeployOpts.CatalogItemID)
	assert.Equal(t, "beta", fx.catalogResp.LastDeployOpts.Name)
}

func TestExecuteDeploy_PickerCancelSurfacesError(t *testing.T) {
	items := []types.GitCatalogItem{{ID: "item-1", Name: "alpha"}}
	fx := newDeployTestFixture(t, items)

	cancelled := errors.New("user cancelled")
	picker := func([]types.GitCatalogItem) (string, error) { return "", cancelled }

	err := fx.cmd.executeDeploy(context.Background(), "", &deployFlags{wait: false}, picker)
	require.ErrorIs(t, err, cancelled)
}

func TestExecuteDeploy_EmptyCatalogReturnsUserError(t *testing.T) {
	fx := newDeployTestFixture(t, nil)
	picker := func([]types.GitCatalogItem) (string, error) {
		t.Fatal("picker should not be called when the catalog is empty")
		return "", nil
	}

	err := fx.cmd.executeDeploy(context.Background(), "", &deployFlags{wait: false}, picker)
	require.Error(t, err)
	assert.ErrorIs(t, err, errNoCatalogItems.E)
}

func TestFormatPickerLabel(t *testing.T) {
	tests := []struct {
		name string
		in   types.GitCatalogItem
		want string
	}{
		{
			name: "full label",
			in:   types.GitCatalogItem{Name: "demo", RepositoryURL: "https://github.com/okteto/movies", Branch: "main"},
			want: "demo — https://github.com/okteto/movies@main",
		},
		{
			name: "no branch defaults to 'default'",
			in:   types.GitCatalogItem{Name: "demo", RepositoryURL: "repo"},
			want: "demo — repo@default",
		},
		{
			name: "no repo shows just the name",
			in:   types.GitCatalogItem{Name: "demo"},
			want: "demo",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, formatPickerLabel(tc.in))
		})
	}
}
