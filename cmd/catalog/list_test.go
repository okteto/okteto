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
	"bytes"
	"testing"

	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateListOutput(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "empty is valid", input: "", wantErr: false},
		{name: "json is valid", input: "json", wantErr: false},
		{name: "yaml is valid", input: "yaml", wantErr: false},
		{name: "other is invalid", input: "xml", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateListOutput(tc.input)
			assert.Equal(t, tc.wantErr, err != nil)
		})
	}
}

func TestToCatalogListOutput_SortsByName(t *testing.T) {
	items := []types.GitCatalogItem{
		{Name: "zeta"},
		{Name: "alpha"},
		{Name: "beta"},
	}
	out := toCatalogListOutput(items)
	require.Len(t, out, 3)
	assert.Equal(t, "alpha", out[0].Name)
	assert.Equal(t, "beta", out[1].Name)
	assert.Equal(t, "zeta", out[2].Name)
}

func TestToCatalogListOutput_ExtractsVariableNames(t *testing.T) {
	items := []types.GitCatalogItem{
		{
			Name: "demo",
			Variables: []types.GitCatalogItemVariable{
				{Name: "FOO", Value: "bar"},
				{Name: "BAZ", Value: "qux"},
			},
		},
	}
	out := toCatalogListOutput(items)
	require.Len(t, out, 1)
	assert.ElementsMatch(t, []string{"FOO", "BAZ"}, out[0].Variables)
}

func TestDisplayCatalogItems_TableEmpty(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, displayCatalogItems(&buf, nil, ""))
	assert.Contains(t, buf.String(), "There are no catalog items available")
}

func TestDisplayCatalogItems_TableWithItems(t *testing.T) {
	items := []types.GitCatalogItem{
		{
			Name:          "demo",
			RepositoryURL: "https://github.com/okteto/movies",
			Branch:        "main",
			ManifestPath:  "okteto.yml",
			ReadOnly:      true,
		},
	}
	var buf bytes.Buffer
	require.NoError(t, displayCatalogItems(&buf, items, ""))
	out := buf.String()
	assert.Contains(t, out, "Name")
	assert.Contains(t, out, "demo")
	assert.Contains(t, out, "https://github.com/okteto/movies")
	assert.Contains(t, out, "main")
	assert.Contains(t, out, "okteto.yml")
	assert.Contains(t, out, "true")
}

func TestDisplayCatalogItems_JSONEmpty(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, displayCatalogItems(&buf, nil, "json"))
	assert.Contains(t, buf.String(), "[]")
}

func TestDisplayCatalogItems_JSONWithItems(t *testing.T) {
	items := []types.GitCatalogItem{
		{Name: "demo", RepositoryURL: "repo", Branch: "main"},
	}
	var buf bytes.Buffer
	require.NoError(t, displayCatalogItems(&buf, items, "json"))
	out := buf.String()
	assert.Contains(t, out, `"name": "demo"`)
	assert.Contains(t, out, `"repositoryUrl": "repo"`)
}

func TestDisplayCatalogItems_YAML(t *testing.T) {
	items := []types.GitCatalogItem{
		{Name: "demo", RepositoryURL: "repo"},
	}
	var buf bytes.Buffer
	require.NoError(t, displayCatalogItems(&buf, items, "yaml"))
	out := buf.String()
	assert.Contains(t, out, "name: demo")
	assert.Contains(t, out, "repositoryUrl: repo")
}

func TestValueOrDash(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "non-empty returned as-is", in: "foo", want: "foo"},
		{name: "empty becomes dash", in: "", want: "-"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, valueOrDash(tc.in))
		})
	}
}
