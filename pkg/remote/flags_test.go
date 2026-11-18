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

	"github.com/kballard/go-shellquote"
	"github.com/stretchr/testify/require"
)

// TestCommandFlag reads each flag back with the shell's rules, so it checks what remote-run receives rather than the
// quoting style
func TestCommandFlag(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected string
	}{
		{name: "plain value", value: "test", expected: "test"},
		{name: "empty value", value: "", expected: ""},
		{name: "spaces", value: "this is a test", expected: "this is a test"},
		{name: "command substitution", value: "$(echo x)", expected: "$(echo x)"},
		{name: "backticks", value: "`echo x`", expected: "`echo x`"},
		{name: "env var reference", value: "$HOME", expected: "$HOME"},
		{name: "single quote", value: "it's", expected: "it's"},
		{name: "double quotes", value: `say "hi"`, expected: `say "hi"`},
		{name: "backslash", value: `C:\path`, expected: `C:\path`},
		{name: "comment and separator", value: "a # b; c", expected: "a # b; c"},
		{name: "tab and carriage return", value: "a\tb\rc", expected: "a\tb\rc"},
		{name: "newline", value: "multi\nline", expected: `multi\nline`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, err := shellquote.Split(CommandFlag("name", tt.value))
			require.NoError(t, err)
			require.Equal(t, []string{"--name", tt.expected}, args)
		})
	}
}

// A newline in a value can't end the RUN instruction and start a new one
func TestCommandFlagKeepsTheRunInstructionInOneLine(t *testing.T) {
	flag := CommandFlag("var", "A=x\nRUN echo injected #")

	require.NotContains(t, flag, "\n")
}
