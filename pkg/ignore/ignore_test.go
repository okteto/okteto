// Copyright 2024 The Okteto Authors
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

package ignore

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIgnore(t *testing.T) {
	type exp struct {
		str   string
		files []string
	}
	tt := []struct {
		expected map[string]exp
		name     string
		input    string
	}{
		{
			name: "basic",
			input: `
		.git
		[deploy]
		integration
		[destroy]
		frontend/*
		backend
		[test]
		chart
		[test.frontend]
		backend
		`,
			expected: map[string]exp{
				RootSection:     {str: "\n.git\n", files: []string{".git"}},
				"deploy":        {str: "integration\n", files: []string{"integration"}},
				"destroy":       {str: "frontend/*\nbackend\n", files: []string{"frontend/*", "backend"}},
				"test":          {str: "chart\n", files: []string{"chart"}},
				"test.frontend": {str: "backend\n\n", files: []string{"backend"}},
			},
		},
		{
			name: "leading_trailing_whitespace",
			input: `
		      .git
		      [deploy]
		      integration
		      [destroy]
		      frontend/*
		      backend
		      [test]
		      chart
		      [test.frontend]
		      backend
		`,
			expected: map[string]exp{
				RootSection:     {str: "\n.git\n", files: []string{".git"}},
				"deploy":        {str: "integration\n", files: []string{"integration"}},
				"destroy":       {str: "frontend/*\nbackend\n", files: []string{"frontend/*", "backend"}},
				"test":          {str: "chart\n", files: []string{"chart"}},
				"test.frontend": {str: "backend\n\n", files: []string{"backend"}},
			},
		},
		{
			name: "extran_newlines",
			input: `

      .git     

      [deploy]     

      integration     


      [destroy]     


      frontend/*     


      backend     


      [test]     

      chart     


      [test.frontend]     

      backend     



`,
			expected: map[string]exp{
				RootSection:     {str: "\n\n.git\n\n", files: []string{".git"}},
				"deploy":        {str: "\nintegration\n\n\n", files: []string{"integration"}},
				"destroy":       {str: "\n\nfrontend/*\n\n\nbackend\n\n\n", files: []string{"frontend/*", "backend"}},
				"test":          {str: "\nchart\n\n\n", files: []string{"chart"}},
				"test.frontend": {str: "\nbackend\n\n\n\n", files: []string{"backend"}},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			ig := NewFromReader(strings.NewReader(tc.input))
			require.Len(t, ig.sections, len(tc.expected))
			for expectedKey, expectedVal := range tc.expected {
				assert.Equal(t, expectedVal.str, ig.Get(expectedKey))
				f, err := ig.Rules(expectedKey)
				assert.NoError(t, err)
				assert.ElementsMatch(t, f, expectedVal.files)
			}
		})
	}
}

func TestMultiRules(t *testing.T) {
	input := `
.git
[deploy]
integration
[destroy]
frontend/*
backend
[test]
chart
[test.frontend]
backend
`
	ig := NewFromReader(strings.NewReader(input))
	f, err := ig.Rules(RootSection, "test", "test.frontend")
	assert.NoError(t, err)
	assert.ElementsMatch(t, f, []string{".git", "chart", "backend"})

}
