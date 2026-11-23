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

package test

import (
	"slices"
	"testing"

	"github.com/okteto/okteto/pkg/dag"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/require"
)

func newGroupedTests() model.ManifestTests {
	return model.ManifestTests{
		"unit":        &model.Test{Groups: []string{"pre-commit"}},
		"lint":        &model.Test{Groups: []string{"pre-commit", "release"}},
		"e2e":         &model.Test{Groups: []string{"long-running"}, DependsOn: []string{"seed"}},
		"integration": &model.Test{Groups: []string{"long-running"}},
		"seed":        &model.Test{},
	}
}

func TestSelectTestsWithoutGroups(t *testing.T) {
	tests := []struct {
		name  string
		names []string
	}{
		{
			name:  "no names",
			names: []string{},
		},
		{
			name:  "names are kept as they are",
			names: []string{"lint", "unit", "lint"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selected, err := selectTests(newGroupedTests(), tt.names, nil)

			require.NoError(t, err)
			require.Equal(t, tt.names, selected)
		})
	}
}

func TestSelectTestsWithGroups(t *testing.T) {
	tests := []struct {
		name     string
		names    []string
		groups   []string
		expected []string
	}{
		{
			name:     "one group",
			groups:   []string{"pre-commit"},
			expected: []string{"lint", "unit"},
		},
		{
			name:     "group with a single test container",
			groups:   []string{"release"},
			expected: []string{"lint"},
		},
		{
			name:     "several groups are joined",
			groups:   []string{"pre-commit", "long-running"},
			expected: []string{"lint", "unit", "e2e", "integration"},
		},
		{
			name:     "test containers in several selected groups are not repeated",
			groups:   []string{"pre-commit", "release"},
			expected: []string{"lint", "unit"},
		},
		{
			name:     "repeated groups are ignored",
			groups:   []string{"release", "release"},
			expected: []string{"lint"},
		},
		{
			name:     "names and groups are joined",
			names:    []string{"e2e"},
			groups:   []string{"pre-commit"},
			expected: []string{"e2e", "lint", "unit"},
		},
		{
			name:     "names that belong to the group are not repeated",
			names:    []string{"unit", "unit"},
			groups:   []string{"pre-commit"},
			expected: []string{"unit", "lint"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selected, err := selectTests(newGroupedTests(), tt.names, tt.groups)

			require.NoError(t, err)
			require.Equal(t, tt.expected, selected)
		})
	}
}

func TestSelectTestsGroupNotFound(t *testing.T) {
	selected, err := selectTests(newGroupedTests(), []string{"unit"}, []string{"pre-commit", "nightly"})

	require.ErrorIs(t, err, errGroupNotFound)
	require.EqualError(t, err, "no test container belongs to the group 'nightly'")
	var userErr oktetoErrors.UserError
	require.ErrorAs(t, err, &userErr)
	require.Equal(t, "The groups defined in your Okteto Manifest are: long-running, pre-commit, release", userErr.Hint)
	require.Nil(t, selected)
}

func TestSelectTestsGroupNameIsCaseSensitive(t *testing.T) {
	_, err := selectTests(newGroupedTests(), nil, []string{"Pre-Commit"})

	require.ErrorIs(t, err, errGroupNotFound)
}

func TestSelectTestsGroupNotFoundWithoutGroupsDefined(t *testing.T) {
	tests := model.ManifestTests{
		"unit": &model.Test{},
	}

	_, err := selectTests(tests, nil, []string{"pre-commit"})

	require.ErrorIs(t, err, errGroupNotFound)
	var userErr oktetoErrors.UserError
	require.ErrorAs(t, err, &userErr)
	require.Equal(t, noGroupsDefinedHint, userErr.Hint)
}

// TestSelectedTestsIncludeDependencies checks that the test containers a group depends on run
// too, and before it, even when they don't belong to the group
func TestSelectedTestsIncludeDependencies(t *testing.T) {
	tests := newGroupedTests()
	nodes := make([]dag.Node, 0, len(tests))
	for name, test := range tests {
		nodes = append(nodes, Node{Test: test, Name: name})
	}
	tree, err := dag.From(nodes...)
	require.NoError(t, err)

	selected, err := selectTests(tests, nil, []string{"long-running"})
	require.NoError(t, err)
	subtree, err := tree.Subtree(selected...)
	require.NoError(t, err)

	ordered := subtree.Ordered()
	require.ElementsMatch(t, []string{"seed", "e2e", "integration"}, ordered)
	require.Less(t, slices.Index(ordered, "seed"), slices.Index(ordered, "e2e"))
}

func TestValidateGroupFlags(t *testing.T) {
	tests := []struct {
		name              string
		groups            []string
		overrideRequested bool
	}{
		{
			name: "no groups",
		},
		{
			name:   "groups",
			groups: []string{"pre-commit", "release"},
		},
		{
			name:              "command override without groups",
			overrideRequested: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, validateGroupFlags(tt.groups, tt.overrideRequested))
		})
	}
}

func TestValidateGroupFlagsErrors(t *testing.T) {
	tests := []struct {
		name              string
		groups            []string
		overrideRequested bool
		expectedErr       error
		expectedHint      string
	}{
		{
			name:         "empty group",
			groups:       []string{"pre-commit", ""},
			expectedErr:  errEmptyGroupFlag,
			expectedHint: groupFlagHint,
		},
		{
			name:         "blank group",
			groups:       []string{" "},
			expectedErr:  errEmptyGroupFlag,
			expectedHint: groupFlagHint,
		},
		{
			name:              "group with command override",
			groups:            []string{"pre-commit"},
			overrideRequested: true,
			expectedErr:       errGroupWithCommandOverride,
			expectedHint:      commandOverrideHint,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGroupFlags(tt.groups, tt.overrideRequested)

			require.ErrorIs(t, err, tt.expectedErr)
			var userErr oktetoErrors.UserError
			require.ErrorAs(t, err, &userErr)
			require.Equal(t, tt.expectedHint, userErr.Hint)
		})
	}
}
