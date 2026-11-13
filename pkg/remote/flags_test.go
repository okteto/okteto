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

package remote

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCommandFlag(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected string
	}{
		{name: "plain value", value: "test", expected: "--name test"},
		{name: "empty value", value: "", expected: "--name ''"},
		{name: "spaces", value: "this is a test", expected: "--name 'this is a test'"},
		{name: "command substitution", value: "$(echo x)", expected: "--name '$(echo x)'"},
		{name: "backticks", value: "`echo x`", expected: "--name '`echo x`'"},
		{name: "env var reference", value: "$HOME", expected: "--name '$HOME'"},
		{name: "single quote", value: "it's", expected: `--name 'it'"'"'s'`},
		{name: "double quotes", value: `say "hi"`, expected: `--name 'say "hi"'`},
		{name: "backslash", value: `C:\path`, expected: `--name 'C:\path'`},
		{name: "newline", value: "multi\nline", expected: `--name 'multi\nline'`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, CommandFlag("name", tt.value))
		})
	}
}

// A newline in a value can't end the RUN instruction and start a new one
func TestCommandFlagKeepsTheRunInstructionInOneLine(t *testing.T) {
	flag := CommandFlag("var", "A=x\nRUN echo injected #")

	require.NotContains(t, flag, "\n")
}
