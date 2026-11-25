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

func newProfiledTests() model.ManifestTests {
	return model.ManifestTests{
		"unit":        &model.Test{Profiles: []string{"pre-commit"}},
		"lint":        &model.Test{Profiles: []string{"pre-commit", "release"}},
		"e2e":         &model.Test{Profiles: []string{"long-running"}, DependsOn: []string{"seed"}},
		"integration": &model.Test{Profiles: []string{"long-running"}},
		"seed":        &model.Test{},
		"vet":         &model.Test{},
	}
}

func TestResolveProfiles(t *testing.T) {
	tests := []struct {
		name            string
		flagProfiles    []string
		envValue        string
		expected        []string
		expectedSource  string
		expectedSet     bool
		expectedFromEnv bool
	}{
		{
			name:           "no profiles",
			expected:       nil,
			expectedSource: profileFlagSource,
		},
		{
			name:           "profiles from the flag",
			flagProfiles:   []string{"pre-commit", "release"},
			expected:       []string{"pre-commit", "release"},
			expectedSource: profileFlagSource,
			expectedSet:    true,
		},
		{
			name:            "profiles from the environment variable",
			envValue:        "pre-commit,release",
			expected:        []string{"pre-commit", "release"},
			expectedSource:  profileEnvSource,
			expectedSet:     true,
			expectedFromEnv: true,
		},
		{
			name:            "the environment variable overrides the flag",
			flagProfiles:    []string{"pre-commit"},
			envValue:        "release",
			expected:        []string{"release"},
			expectedSource:  profileEnvSource,
			expectedSet:     true,
			expectedFromEnv: true,
		},
		{
			name:            "the environment variable overrides an empty flag value",
			flagProfiles:    []string{""},
			envValue:        "release",
			expected:        []string{"release"},
			expectedSource:  profileEnvSource,
			expectedSet:     true,
			expectedFromEnv: true,
		},
		{
			name:            "spaces and empty values of the environment variable are ignored",
			envValue:        " pre-commit , ,release,",
			expected:        []string{"pre-commit", "release"},
			expectedSource:  profileEnvSource,
			expectedSet:     true,
			expectedFromEnv: true,
		},
		{
			name:           "an environment variable without profiles doesn't override the flag",
			flagProfiles:   []string{"pre-commit"},
			envValue:       " , ",
			expected:       []string{"pre-commit"},
			expectedSource: profileFlagSource,
			expectedSet:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selection, err := resolveProfiles(tt.flagProfiles, tt.envValue)

			require.NoError(t, err)
			require.Equal(t, tt.expected, selection.profiles)
			require.Equal(t, tt.expectedSource, selection.source)
			require.Equal(t, tt.expectedFromEnv, selection.fromEnv())
			require.Equal(t, tt.expectedSet, selection.isSet())
		})
	}
}

func TestResolveProfilesRejectsEmptyFlag(t *testing.T) {
	tests := []struct {
		name         string
		flagProfiles []string
	}{
		{
			name:         "empty profile",
			flagProfiles: []string{"pre-commit", ""},
		},
		{
			name:         "blank profile",
			flagProfiles: []string{" "},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolveProfiles(tt.flagProfiles, "")

			require.ErrorIs(t, err, errEmptyProfileFlag)
			var userErr oktetoErrors.UserError
			require.ErrorAs(t, err, &userErr)
			require.Equal(t, profileFlagHint, userErr.Hint)
		})
	}
}

func TestProfileSelectionWithoutConflicts(t *testing.T) {
	tests := []struct {
		name              string
		selection         profileSelection
		names             []string
		overrideRequested bool
	}{
		{
			name:              "test names and command override without profiles",
			selection:         profileSelection{source: profileFlagSource},
			names:             []string{"unit"},
			overrideRequested: true,
		},
		{
			name:      "profiles without test names or command override",
			selection: profileSelection{profiles: []string{"pre-commit"}, source: profileEnvSource},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, tt.selection.checkCommandOverride(tt.overrideRequested))
			require.NoError(t, tt.selection.checkTestNames(tt.names))
		})
	}
}

func TestProfileSelectionCheckCommandOverride(t *testing.T) {
	tests := []struct {
		name         string
		source       string
		expectedErr  string
		expectedHint string
	}{
		{
			name:         "profiles from the flag",
			source:       profileFlagSource,
			expectedErr:  "'--profile' cannot be used when overriding the commands of a test container",
			expectedHint: commandOverrideHint,
		},
		{
			name:         "profiles from the environment variable",
			source:       profileEnvSource,
			expectedErr:  "'OKTETO_PROFILES' cannot be used when overriding the commands of a test container",
			expectedHint: profileEnvConflictHint,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selection := profileSelection{profiles: []string{"pre-commit"}, source: tt.source}

			err := selection.checkCommandOverride(true)

			require.ErrorIs(t, err, errProfileWithCommandOverride)
			require.EqualError(t, err, tt.expectedErr)
			var userErr oktetoErrors.UserError
			require.ErrorAs(t, err, &userErr)
			require.Equal(t, tt.expectedHint, userErr.Hint)
		})
	}
}

func TestProfileSelectionCheckTestNames(t *testing.T) {
	tests := []struct {
		name         string
		source       string
		expectedErr  string
		expectedHint string
	}{
		{
			name:         "profiles from the flag",
			source:       profileFlagSource,
			expectedErr:  "'--profile' cannot be used when naming the test containers to run",
			expectedHint: profileWithTestNamesHint,
		},
		{
			name:         "profiles from the environment variable",
			source:       profileEnvSource,
			expectedErr:  "'OKTETO_PROFILES' cannot be used when naming the test containers to run",
			expectedHint: profileEnvConflictHint,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selection := profileSelection{profiles: []string{"pre-commit"}, source: tt.source}

			err := selection.checkTestNames([]string{"unit"})

			require.ErrorIs(t, err, errProfileWithTestNames)
			require.EqualError(t, err, tt.expectedErr)
			var userErr oktetoErrors.UserError
			require.ErrorAs(t, err, &userErr)
			require.Equal(t, tt.expectedHint, userErr.Hint)
		})
	}
}

// TestSelectTestsByName checks that named test containers run whatever their profiles
func TestSelectTestsByName(t *testing.T) {
	names := []string{"e2e", "unit", "e2e"}

	selected, err := selectTests(newProfiledTests(), names, nil)

	require.NoError(t, err)
	require.Equal(t, names, selected)
}

func TestSelectTestsByProfile(t *testing.T) {
	tests := []struct {
		name     string
		profiles []string
		expected []string
	}{
		{
			name:     "no profiles runs the test containers without profiles",
			expected: []string{"seed", "vet"},
		},
		{
			name:     "one profile",
			profiles: []string{"pre-commit"},
			expected: []string{"lint", "seed", "unit", "vet"},
		},
		{
			name:     "profile with a single test container",
			profiles: []string{"release"},
			expected: []string{"lint", "seed", "vet"},
		},
		{
			name:     "several profiles are joined",
			profiles: []string{"pre-commit", "long-running"},
			expected: []string{"e2e", "integration", "lint", "seed", "unit", "vet"},
		},
		{
			name:     "test containers with several enabled profiles are not repeated",
			profiles: []string{"pre-commit", "release"},
			expected: []string{"lint", "seed", "unit", "vet"},
		},
		{
			name:     "repeated profiles are ignored",
			profiles: []string{"release", "release"},
			expected: []string{"lint", "seed", "vet"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selected, err := selectTests(newProfiledTests(), nil, tt.profiles)

			require.NoError(t, err)
			require.Equal(t, tt.expected, selected)
		})
	}
}

func TestSelectTestsWithoutProfilesDefined(t *testing.T) {
	tests := model.ManifestTests{
		"unit": &model.Test{},
		"e2e":  &model.Test{DependsOn: []string{"unit"}},
	}

	selected, err := selectTests(tests, nil, nil)

	require.NoError(t, err)
	require.Equal(t, []string{"e2e", "unit"}, selected)
}

func TestSelectTestsProfileNotFound(t *testing.T) {
	selected, err := selectTests(newProfiledTests(), nil, []string{"pre-commit", "nightly"})

	require.ErrorIs(t, err, errProfileNotFound)
	require.EqualError(t, err, "no test container has the profile 'nightly'")
	var userErr oktetoErrors.UserError
	require.ErrorAs(t, err, &userErr)
	require.Equal(t, "The profiles defined in your Okteto Manifest are: long-running, pre-commit, release", userErr.Hint)
	require.Nil(t, selected)
}

func TestSelectTestsProfileNameIsCaseSensitive(t *testing.T) {
	_, err := selectTests(newProfiledTests(), nil, []string{"Pre-Commit"})

	require.ErrorIs(t, err, errProfileNotFound)
}

func TestSelectTestsProfileNotFoundWithoutProfilesDefined(t *testing.T) {
	tests := model.ManifestTests{
		"unit": &model.Test{},
	}

	_, err := selectTests(tests, nil, []string{"pre-commit"})

	require.ErrorIs(t, err, errProfileNotFound)
	var userErr oktetoErrors.UserError
	require.ErrorAs(t, err, &userErr)
	require.Equal(t, noProfilesDefinedHint, userErr.Hint)
}

// TestSelectTestsAllTestsHaveProfiles checks that running without profiles fails when every test
// container has a profile, instead of running nothing or all of them
func TestSelectTestsAllTestsHaveProfiles(t *testing.T) {
	tests := model.ManifestTests{
		"unit": &model.Test{Profiles: []string{"pre-commit"}},
		"e2e":  &model.Test{Profiles: []string{"long-running", "release"}},
	}

	selected, err := selectTests(tests, nil, nil)

	require.ErrorIs(t, err, errNoTestsWithoutProfiles)
	var userErr oktetoErrors.UserError
	require.ErrorAs(t, err, &userErr)
	require.Equal(t, `Enable a profile with '--profile' or the OKTETO_PROFILES environment variable, for example:
    okteto test --profile <name>
    The profiles defined in your Okteto Manifest are: long-running, pre-commit, release`, userErr.Hint)
	require.Nil(t, selected)
}

// TestSelectTestsAllTestsHaveEnabledProfiles checks that every test container runs when all of
// them have profiles and one of their profiles is enabled
func TestSelectTestsAllTestsHaveEnabledProfiles(t *testing.T) {
	tests := model.ManifestTests{
		"unit": &model.Test{Profiles: []string{"release"}},
		"e2e":  &model.Test{Profiles: []string{"long-running", "release"}},
	}

	selected, err := selectTests(tests, nil, []string{"release"})

	require.NoError(t, err)
	require.Equal(t, []string{"e2e", "unit"}, selected)
}

// TestSelectTestsDependencyNotEnabled checks that, as in Docker Compose, a selected test container
// cannot depend on one whose profiles are not enabled
func TestSelectTestsDependencyNotEnabled(t *testing.T) {
	tests := []struct {
		name        string
		tests       model.ManifestTests
		profiles    []string
		expectedErr string
	}{
		{
			name: "dependency with another profile",
			tests: model.ManifestTests{
				"smoke":   &model.Test{Profiles: []string{"release"}, DependsOn: []string{"seed-db"}},
				"seed-db": &model.Test{Profiles: []string{"long-running", "nightly"}},
			},
			profiles:    []string{"release"},
			expectedErr: "test container 'smoke' depends on 'seed-db', but none of its profiles is enabled: long-running, nightly",
		},
		{
			name: "test container without profiles depends on one with profiles",
			tests: model.ManifestTests{
				"e2e":  &model.Test{DependsOn: []string{"seed"}},
				"seed": &model.Test{Profiles: []string{"data"}},
			},
			expectedErr: "test container 'e2e' depends on 'seed', but none of its profiles is enabled: data",
		},
		{
			name: "dependency of an enabled dependency",
			tests: model.ManifestTests{
				"e2e":     &model.Test{Profiles: []string{"release"}, DependsOn: []string{"seed"}},
				"seed":    &model.Test{DependsOn: []string{"migrate"}},
				"migrate": &model.Test{Profiles: []string{"data"}},
			},
			profiles:    []string{"release"},
			expectedErr: "test container 'seed' depends on 'migrate', but none of its profiles is enabled: data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selected, err := selectTests(tt.tests, nil, tt.profiles)

			require.ErrorIs(t, err, errDependencyProfilesNotEnabled)
			require.EqualError(t, err, tt.expectedErr)
			var userErr oktetoErrors.UserError
			require.ErrorAs(t, err, &userErr)
			require.Equal(t, dependencyProfilesHint, userErr.Hint)
			require.Nil(t, selected)
		})
	}
}

func TestSelectTestsDependencyEnabled(t *testing.T) {
	tests := []struct {
		name     string
		tests    model.ManifestTests
		profiles []string
		expected []string
	}{
		{
			name: "dependency with another enabled profile",
			tests: model.ManifestTests{
				"smoke":   &model.Test{Profiles: []string{"release"}, DependsOn: []string{"seed-db"}},
				"seed-db": &model.Test{Profiles: []string{"long-running"}},
			},
			profiles: []string{"release", "long-running"},
			expected: []string{"seed-db", "smoke"},
		},
		{
			name: "dependency with the same profile",
			tests: model.ManifestTests{
				"smoke":   &model.Test{Profiles: []string{"release"}, DependsOn: []string{"seed-db"}},
				"seed-db": &model.Test{Profiles: []string{"release", "long-running"}},
			},
			profiles: []string{"release"},
			expected: []string{"seed-db", "smoke"},
		},
		{
			name: "dependency of a test container that doesn't run",
			tests: model.ManifestTests{
				"smoke":   &model.Test{Profiles: []string{"release"}, DependsOn: []string{"seed-db"}},
				"seed-db": &model.Test{Profiles: []string{"long-running"}},
				"unit":    &model.Test{},
			},
			expected: []string{"unit"},
		},
		{
			name: "unknown dependencies are left to the dependency tree",
			tests: model.ManifestTests{
				"smoke": &model.Test{DependsOn: []string{"unknown"}},
			},
			expected: []string{"smoke"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selected, err := selectTests(tt.tests, nil, tt.profiles)

			require.NoError(t, err)
			require.Equal(t, tt.expected, selected)
		})
	}
}

// TestSelectedTestsRunAfterTheirDependencies checks that the test containers of a profile run
// after the test containers they depend on
func TestSelectedTestsRunAfterTheirDependencies(t *testing.T) {
	tests := newProfiledTests()
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
	require.ElementsMatch(t, []string{"e2e", "integration", "seed", "vet"}, ordered)
	require.Less(t, slices.Index(ordered, "seed"), slices.Index(ordered, "e2e"))
}
