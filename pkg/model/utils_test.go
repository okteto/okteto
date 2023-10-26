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

package model

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GetValidNameFromFolder(t *testing.T) {
	var tests = []struct {
		name     string
		folder   string
		expected string
	}{
		{name: "all lower case", folder: "lowercase", expected: "lowercase"},
		{name: "with some lower case", folder: "lowerCase", expected: "lowerCase"},
		{name: "upper case", folder: "UpperCase", expected: "UpperCase"},
		{name: "valid symbols", folder: "getting-started.test", expected: "getting-started.test"},
		{name: "invalid symbols", folder: "getting_$#started", expected: "getting_$#started"},
		{name: "current folder", folder: ".", expected: "model"},
		{name: "parent folder", folder: "..", expected: "pkg"},
		{name: "okteto folder", folder: ".okteto", expected: "model"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := GetValidNameFromFolder(tt.folder)
			if err != nil {
				t.Errorf("got an error in '%s': %s", tt.name, err)
			}
			if actual != tt.expected {
				t.Errorf("'%s' got '%s' expected '%s'", tt.name, actual, tt.expected)
			}
		})
	}
}

func Test_GetValidNameFromGitRepo(t *testing.T) {
	var tests = []struct {
		name     string
		gitRepo  string
		expected string
	}{
		{name: "https url", gitRepo: "https://github.com/okteto/stacks-getting-started", expected: "stacks-getting-started"},
		{name: "https with slash at the end", gitRepo: "https://github.com/okteto/stacks-getting-started/", expected: "stacks-getting-started"},
		{name: "ssh url", gitRepo: "git@github.com:okteto/stacks-getting-started.git", expected: "stacks-getting-started"},
		{name: "ssh url with slash at the end", gitRepo: "git@github.com:okteto/stacks-getting-started.git/", expected: "stacks-getting-started"},
		{name: "https with dots", gitRepo: "https://github.com/okteto/stacks.getting.started", expected: "stacks.getting.started"},
		{name: "URL with uppers", gitRepo: "https://github.com/okteto/StacksGettingStarted", expected: "StacksGettingStarted"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TranslateURLToName(tt.gitRepo)

			if result != tt.expected {
				t.Errorf("'%s' got '%s' expected '%s'", tt.name, result, tt.expected)
			}
		})
	}

}

func TestGetCycles(t *testing.T) {
	var tests = []struct {
		name          string
		g             graph
		expectedCycle bool
	}{
		{
			name: "no cycle - no connections",
			g: graph{
				"a": []string{},
				"b": []string{},
				"c": []string{},
			},
			expectedCycle: false,
		},
		{
			name: "no cycle - connections",
			g: graph{
				"a": []string{"b"},
				"b": []string{"c"},
				"c": []string{},
			},
			expectedCycle: false,
		},
		{
			name: "cycle - direct cycle",
			g: graph{
				"a": []string{"b"},
				"b": []string{"a"},
				"c": []string{},
			},
			expectedCycle: true,
		},
		{
			name: "cycle - indirect cycle",
			g: graph{
				"a": []string{"b"},
				"b": []string{"c"},
				"c": []string{"a"},
			},
			expectedCycle: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getDependentCyclic(tt.g)
			assert.Equal(t, tt.expectedCycle, len(result) > 0)
		})
	}

}

func Test_snapshotEnv(t *testing.T) {
	os.Clearenv()

	vars := map[string]string{
		"TEST_1": "1",
		"TEST_2": "2",
	}
	for k, v := range vars {
		t.Setenv(k, v)
	}

	env := snapshotEnv()

	expected := map[string]string{
		"TEST_1": "1",
		"TEST_2": "2",
	}
	assert.Equal(t, expected, env)
}

func Test_restoreEnv(t *testing.T) {
	os.Clearenv()

	vars := map[string]string{
		"TEST_1": "1",
		"TEST_2": "2",
	}
	for k, v := range vars {
		t.Setenv(k, v)
	}

	env := snapshotEnv()
	os.Clearenv()

	assert.Equal(t, os.Environ(), []string{})

	t.Setenv("TEST_3", "3")
	assert.NoError(t, restoreEnv(env))

	checks := []struct {
		key    string
		exists bool
	}{
		{"TEST_1", true},
		{"TEST_2", true},
		{"TEST_3", false},
	}

	for _, c := range checks {
		_, has := os.LookupEnv(c.key)
		assert.Equal(t, has, c.exists)
	}
}
