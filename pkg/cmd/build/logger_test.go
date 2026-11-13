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

package build

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// testStep returns the name BuildKit gives to the remote-run test step of the generated Dockerfile
func testStep(flags string) string {
	return `[runner 9/9] RUN --mount=type=secret,id=known_hosts mkdir -p $HOME/.ssh && ` +
		`/okteto/bin/okteto remote-run test --log-output=json --server-name="$INTERNAL_SERVER_NAME" ` + flags + ` || true`
}

func TestTestNameFromStep(t *testing.T) {
	tests := []struct {
		name     string
		step     string
		expected string
		found    bool
	}{
		{
			name:     "plain name",
			step:     testStep(`--name unit`),
			expected: "unit",
			found:    true,
		},
		{
			name:     "name with spaces",
			step:     testStep(`--name 'my test'`),
			expected: "my test",
			found:    true,
		},
		{
			name:     "name with a variable",
			step:     testStep(`--name '$HOME'`),
			expected: "$HOME",
			found:    true,
		},
		{
			name:     "name with a single quote",
			step:     testStep(`--name 'it'"'"'s'`),
			expected: "it's",
			found:    true,
		},
		{
			name:     "name followed by a var that holds --name",
			step:     testStep(`--name unit --var 'A=--name other'`),
			expected: "unit",
			found:    true,
		},
		{
			name:  "no name",
			step:  testStep(`--var A=b`),
			found: false,
		},
		{
			name:  "unterminated quote",
			step:  `[runner 9/9] RUN /okteto/bin/okteto remote-run test --name 'my te`,
			found: false,
		},
		{
			name:  "not a test step",
			step:  `[runner 9/9] RUN /okteto/bin/okteto remote-run deploy --name unit`,
			found: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, found := testNameFromStep(tt.step)
			assert.Equal(t, tt.expected, name)
			assert.Equal(t, tt.found, found)
		})
	}
}
