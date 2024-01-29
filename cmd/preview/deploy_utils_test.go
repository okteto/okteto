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

package preview

import (
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/assert"
)

func Test_optionsSetup(t *testing.T) {
	ctxUsername := "username"
	okteto.CurrentStore = &okteto.ContextStore{
		CurrentContext: "test",
		Contexts: map[string]*okteto.Context{
			"test": {
				Username: ctxUsername,
			},
		},
	}
	tests := []struct {
		expectError     error
		opts            *DeployOptions
		expectedOptions *DeployOptions
		name            string
		expectedFile    string
		args            []string
	}{
		{
			name: "success-empty-args",
			opts: &DeployOptions{
				scope:      "personal",
				repository: "test-repository",
				branch:     "test-branch",
			},
		},
		{
			name: "success-args",
			args: []string{"preview-name"},
			opts: &DeployOptions{
				scope:      "personal",
				repository: "test-repository",
				branch:     "test-branch",
			},
		},
		{
			name: "success-filename",
			opts: &DeployOptions{
				scope:              "personal",
				repository:         "test-repository",
				branch:             "test-branch",
				deprecatedFilename: "filename-old",
			},
			expectedFile: "filename-old",
		},
		{
			name: "success-filename-file",
			opts: &DeployOptions{
				scope:              "personal",
				repository:         "test-repository",
				branch:             "test-branch",
				deprecatedFilename: "filename-old",
				file:               "file",
			},
			expectedFile: "file",
		},
		{
			name: "get-repository-err",
			opts: &DeployOptions{
				scope:  "personal",
				branch: "test-branch",
			},
			expectError: git.ErrRepositoryNotExists,
		},
		{
			name: "get-branch-err",
			opts: &DeployOptions{
				scope:      "personal",
				repository: "test",
			},
			expectError: git.ErrRepositoryNotExists,
		},
		{
			name: "invalid-scope",
			opts: &DeployOptions{
				scope:      "",
				repository: "test-repository",
				branch:     "test-branch",
			},
			expectError: ErrNotValidPreviewScope,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cwd := t.TempDir()
			err := optionsSetup(cwd, tt.opts, tt.args)
			assert.ErrorIs(t, err, tt.expectError)

			assert.Equal(t, tt.expectedFile, tt.opts.file)
		})
	}

}

func Test_getPreviewURL(t *testing.T) {
	ctxName := "https://my.okteto.instance"
	okteto.CurrentStore = &okteto.ContextStore{
		CurrentContext: "test",
		Contexts: map[string]*okteto.Context{
			"test": {
				Name: ctxName,
			},
		},
	}

	t.Run("full-previews-url", func(t *testing.T) {
		expected := "https://my.okteto.instance/previews/foo-bar"
		actual := getPreviewURL("foo-bar")
		assert.Equal(t, expected, actual)
	})
}
