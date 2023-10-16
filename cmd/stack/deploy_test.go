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

package stack

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
)

func Test_SplitComposeEnv(t *testing.T) {
	sep := ":"
	if runtime.GOOS == "windows" {
		sep = ";"
	}
	var tests = []struct {
		name     string
		envvar   string
		expected []string
	}{
		{
			name:     "only one element",
			envvar:   "/usr/app",
			expected: []string{"/usr/app"},
		},
		{
			name:     "split",
			envvar:   fmt.Sprintf("/usr/app%s/usr/src", sep),
			expected: []string{"/usr/app", "/usr/src"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitComposeFileEnv(tt.envvar)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_loadPath(t *testing.T) {
	sep := ":"
	if runtime.GOOS == "windows" {
		sep = ";"
	}
	var tests = []struct {
		name          string
		stackPath     []string
		composeEnvVar string
		expected      []string
	}{

		{
			name:          "Stackpath is empty and compose file is set",
			stackPath:     []string{},
			composeEnvVar: "/usr/app",
			expected:      []string{"/usr/app"},
		},
		{
			name:          "Stackpath is empty and compose file is set with more than one path",
			stackPath:     []string{},
			composeEnvVar: fmt.Sprintf("/usr/app%s/usr/src", sep),
			expected:      []string{"/usr/app", "/usr/src"},
		},
		{
			name:          "Stackpath is set and compose file is set",
			stackPath:     []string{"/usr/app", "/usr/src"},
			composeEnvVar: fmt.Sprintf("/test/app%s/test/src", sep),
			expected:      []string{"/usr/app", "/usr/src"},
		},
		{
			name:          "Stackpath is set and compose file is notset",
			stackPath:     []string{"/usr/app", "/usr/src"},
			composeEnvVar: "",
			expected:      []string{"/usr/app", "/usr/src"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(model.ComposeFileEnvVar, tt.composeEnvVar)
			result := loadComposePaths(tt.stackPath)
			assert.Equal(t, tt.expected, result)
		})
	}
}
