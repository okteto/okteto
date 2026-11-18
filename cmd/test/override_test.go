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
	"testing"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/require"
)

const noDash = -1

func TestParseTestArgsWithoutOverride(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		argsLenAtDash int
		expectedNames []string
	}{
		{
			name:          "no arguments",
			args:          []string{},
			argsLenAtDash: noDash,
			expectedNames: []string{},
		},
		{
			name:          "test names",
			args:          []string{"lint", "unit"},
			argsLenAtDash: noDash,
			expectedNames: []string{"lint", "unit"},
		},
		{
			name:          "empty test names are filtered out",
			args:          []string{"", "unit", ""},
			argsLenAtDash: noDash,
			expectedNames: []string{"unit"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			names, override, err := parseTestArgs(tt.args, tt.argsLenAtDash, nil)

			require.NoError(t, err)
			require.Equal(t, tt.expectedNames, names)
			require.Equal(t, commandOverride{}, override)
			require.False(t, override.isSet())
		})
	}
}

func TestParseTestArgsWithOverride(t *testing.T) {
	tests := []struct {
		name             string
		args             []string
		argsLenAtDash    int
		commandFlags     []string
		expectedCommands []model.TestCommand
	}{
		{
			name:          "arguments after dash are joined in a single command",
			args:          []string{"unit", "go", "test", "./..."},
			argsLenAtDash: 1,
			expectedCommands: []model.TestCommand{
				{Name: "go test ./...", Command: "go test ./..."},
			},
		},
		{
			name:          "arguments after dash are quoted",
			args:          []string{"unit", "go", "test", "-run", "TestA|TestB", "./my dir"},
			argsLenAtDash: 1,
			expectedCommands: []model.TestCommand{
				{Name: "go test -run 'TestA|TestB' './my dir'", Command: "go test -run 'TestA|TestB' './my dir'"},
			},
		},
		{
			name:          "empty test names before dash are filtered out",
			args:          []string{"", "unit", "echo"},
			argsLenAtDash: 2,
			expectedCommands: []model.TestCommand{
				{Name: "echo", Command: "echo"},
			},
		},
		{
			name:          "command flags are kept as is and in order",
			args:          []string{"unit"},
			argsLenAtDash: noDash,
			commandFlags:  []string{"go mod download", "go test ./... && echo done"},
			expectedCommands: []model.TestCommand{
				{Name: "go mod download", Command: "go mod download"},
				{Name: "go test ./... && echo done", Command: "go test ./... && echo done"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			names, override, err := parseTestArgs(tt.args, tt.argsLenAtDash, tt.commandFlags)

			require.NoError(t, err)
			require.Equal(t, []string{"unit"}, names)
			require.Equal(t, commandOverride{testName: "unit", commands: tt.expectedCommands}, override)
		})
	}
}

// TestParseTestArgsQuotesDashArgs validates that the arguments after '--' are joined into a
// single command where each argument reaches the program unchanged through the remote shell
func TestParseTestArgsQuotesDashArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "safe words are not quoted",
			args:     []string{"go", "test", "-count=1", "./pkg/api/...", "user@host:8080,a+b%c"},
			expected: "go test -count=1 ./pkg/api/... user@host:8080,a+b%c",
		},
		{
			name:     "spaces and shell operators are quoted",
			args:     []string{"go", "test", "-run", "TestA|TestB", "./my dir", "a&&b", "x;y", "*.go"},
			expected: "go test -run 'TestA|TestB' './my dir' 'a&&b' 'x;y' '*.go'",
		},
		{
			name:     "leading hash is quoted so it is not a comment",
			args:     []string{"pytest", "-k", "#smoke", "-v"},
			expected: "pytest -k '#smoke' -v",
		},
		{
			name:     "variables are not expanded remotely",
			args:     []string{"echo", "$HOME", "${OKTETO_NAMESPACE}", "`id`"},
			expected: "echo '$HOME' '${OKTETO_NAMESPACE}' '`id`'",
		},
		{
			name:     "single quotes are escaped",
			args:     []string{"echo", "it's"},
			expected: `echo 'it'"'"'s'`,
		},
		{
			name:     "empty argument is kept",
			args:     []string{"printf", "%s", ""},
			expected: "printf %s ''",
		},
		{
			name:     "shell command is kept as a single argument",
			args:     []string{"sh", "-c", "echo $OKTETO_NAMESPACE && ls"},
			expected: "sh -c 'echo $OKTETO_NAMESPACE && ls'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, override, err := parseTestArgs(append([]string{"unit"}, tt.args...), 1, nil)
			require.NoError(t, err)
			require.Equal(t, []model.TestCommand{{Name: tt.expected, Command: tt.expected}}, override.commands)
		})
	}
}

func TestParseTestArgsErrors(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		argsLenAtDash int
		commandFlags  []string
		expectedErr   error
	}{
		{
			name:          "dash and command flag together",
			args:          []string{"unit", "go", "test"},
			argsLenAtDash: 1,
			commandFlags:  []string{"go test"},
			expectedErr:   errCommandOverrideConflict,
		},
		{
			name:          "dash without command",
			args:          []string{"unit"},
			argsLenAtDash: 1,
			expectedErr:   errEmptyCommandAfterDash,
		},
		{
			name:          "dash with blank arguments",
			args:          []string{"unit", "", " "},
			argsLenAtDash: 1,
			expectedErr:   errEmptyCommandAfterDash,
		},
		{
			name:          "blank command flag",
			args:          []string{"unit"},
			argsLenAtDash: noDash,
			commandFlags:  []string{"go test ./...", " "},
			expectedErr:   errEmptyCommandFlag,
		},
		{
			name:          "dash without test name",
			args:          []string{"go", "test"},
			argsLenAtDash: 0,
			expectedErr:   errCommandOverrideTarget,
		},
		{
			name:          "dash with only an empty test name",
			args:          []string{"", "go", "test"},
			argsLenAtDash: 1,
			expectedErr:   errCommandOverrideTarget,
		},
		{
			name:          "command flag without test name",
			args:          []string{},
			argsLenAtDash: noDash,
			commandFlags:  []string{"go test ./..."},
			expectedErr:   errCommandOverrideTarget,
		},
		{
			name:          "dash with several test names",
			args:          []string{"lint", "unit", "go", "test"},
			argsLenAtDash: 2,
			expectedErr:   errCommandOverrideTarget,
		},
		{
			name:          "command flag with several test names",
			args:          []string{"lint", "unit"},
			argsLenAtDash: noDash,
			commandFlags:  []string{"go test ./..."},
			expectedErr:   errCommandOverrideTarget,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			names, override, err := parseTestArgs(tt.args, tt.argsLenAtDash, tt.commandFlags)

			require.ErrorIs(t, err, tt.expectedErr)
			require.Nil(t, names)
			require.Equal(t, commandOverride{}, override)
		})
	}
}

func TestParseTestArgsReturnsUserErrorWithHint(t *testing.T) {
	_, _, err := parseTestArgs([]string{"lint", "unit", "go", "test"}, 2, nil)

	var userErr oktetoErrors.UserError
	require.ErrorAs(t, err, &userErr)
	require.Equal(t, commandOverrideHint, userErr.Hint)
	require.EqualError(t, err, "overriding the commands requires exactly one test container name, got 2")
}

func TestCommandOverrideAppliesTo(t *testing.T) {
	override := commandOverride{
		testName: "unit",
		commands: []model.TestCommand{{Name: "echo", Command: "echo"}},
	}
	tests := []struct {
		name     string
		override commandOverride
		testName string
		expected bool
	}{
		{
			name:     "overridden test",
			override: override,
			testName: "unit",
			expected: true,
		},
		{
			name:     "other test",
			override: override,
			testName: "lint",
			expected: false,
		},
		{
			name:     "no override",
			override: commandOverride{},
			testName: "unit",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.override.appliesTo(tt.testName))
		})
	}
}

func TestResolveTestOverridesTarget(t *testing.T) {
	manifestCommands := []model.TestCommand{{Name: "go test ./...", Command: "go test ./..."}}
	test := &model.Test{
		Image:               "golang:1.23",
		Context:             "api",
		Commands:            manifestCommands,
		DependsOn:           []string{"lint"},
		Caches:              []string{"/root/.cache/go-build"},
		Artifacts:           []model.Artifact{{Path: "coverage.txt", Destination: "coverage.txt"}},
		Hosts:               []model.Host{{Hostname: "db", IP: "10.0.0.1"}},
		SkipIfNoFileChanges: true,
	}
	overrideCommands := []model.TestCommand{{Name: "go test ./pkg/api", Command: "go test ./pkg/api"}}
	override := commandOverride{testName: "unit", commands: overrideCommands}

	resolved := override.resolveTest("unit", test)

	expected := *test
	expected.Commands = overrideCommands
	require.Equal(t, &expected, resolved)
	require.NotSame(t, test, resolved)
	require.Equal(t, manifestCommands, test.Commands)
}

func TestResolveTestKeepsOtherTests(t *testing.T) {
	test := &model.Test{
		Image:    "golang:1.23",
		Commands: []model.TestCommand{{Name: "golangci-lint run", Command: "golangci-lint run"}},
	}
	override := commandOverride{
		testName: "unit",
		commands: []model.TestCommand{{Name: "go test ./pkg/api", Command: "go test ./pkg/api"}},
	}

	resolved := override.resolveTest("lint", test)

	require.Same(t, test, resolved)
}

func TestResolveTestWithoutOverride(t *testing.T) {
	test := &model.Test{
		Image:    "golang:1.23",
		Commands: []model.TestCommand{{Name: "go test ./...", Command: "go test ./..."}},
	}

	resolved := commandOverride{}.resolveTest("unit", test)

	require.Same(t, test, resolved)
}
