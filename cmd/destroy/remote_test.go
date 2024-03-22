// Copyright 2023 The Okteto Authors
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

package destroy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetCommandFlags(t *testing.T) {
	type config struct {
		opts *Options
	}
	var tests = []struct {
		name     string
		config   config
		expected []string
	}{
		{
			name: "name set",
			config: config{
				opts: &Options{
					Name: "test",
				},
			},
			expected: []string{"--name \"test\""},
		},
		{
			name: "name multiple words",
			config: config{
				opts: &Options{
					Name: "this is a test",
				},
			},
			expected: []string{"--name \"this is a test\""},
		},
		{
			name: "force destroy set",
			config: config{
				opts: &Options{
					Name:         "test",
					ForceDestroy: true,
				},
			},
			expected: []string{"--name \"test\"", "--force-destroy"},
		},
		{
			name: "variables set",
			config: config{
				opts: &Options{
					Name: "test",
					Variables: []string{
						"a=b",
						"c=d",
					},
				},
			},
			expected: []string{"--name \"test\"", "--var a=\"b\" --var c=\"d\""},
		},
		{
			name: "multiword var value",
			config: config{
				opts: &Options{
					Name:      "test",
					Variables: []string{"test=multi word value"},
				},
			},
			expected: []string{"--name \"test\"", "--var test=\"multi word value\""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags, err := getCommandFlags(tt.config.opts)
			assert.Equal(t, tt.expected, flags)
			assert.NoError(t, err)
		})
	}
}
