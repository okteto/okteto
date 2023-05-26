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

package repository

import (
	"context"
	"github.com/go-git/go-git/v5"
	"net/url"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/stretchr/testify/assert"
)

type fakeRepositoryGetter struct {
	repository []*fakeRepository
	err        []error
	callCount  int
}

func (frg *fakeRepositoryGetter) get(_ string) (gitRepositoryInterface, error) {
	i := frg.callCount
	frg.callCount++
	if frg.err != nil && frg.err[i] != nil {
		return nil, frg.err[i]
	}
	return frg.repository[i], nil
}

type fakeRepository struct {
	worktree *fakeWorktree
	head     *plumbing.Reference
	err      error
}

func (fr fakeRepository) Worktree() (gitWorktreeInterface, error) {
	return fr.worktree, fr.err
}

func (fr fakeRepository) Head() (*plumbing.Reference, error) {
	return fr.head, fr.err
}

type fakeWorktree struct {
	status oktetoGitStatus
	root   string
	err    error
}

func (fw fakeWorktree) GetRoot() string {
	return fw.root
}

func (fw fakeWorktree) Status(context.Context, LocalGitInterface) (oktetoGitStatus, error) {
	return fw.status, fw.err
}

func (fs fakeStatus) Status() git.Status {
	return fs.status
}

type fakeStatus struct {
	isClean bool
	status  git.Status
}

func (fs fakeStatus) IsClean() bool {
	return fs.isClean
}

func TestNewRepo(t *testing.T) {
	tt := []struct {
		name            string
		GitCommit       string
		remoteDeploy    string
		expectedControl repositoryInterface
	}{
		{
			name:      "GitCommit is empty",
			GitCommit: "",
			expectedControl: gitRepoController{
				repoGetter: gitRepositoryGetter{},
			},
		},
		{
			name:      "GitCommit is not empty",
			GitCommit: "1234567890",
			expectedControl: gitRepoController{
				repoGetter: gitRepositoryGetter{},
			},
		},
		{
			name:         "GitCommit is not empty in remote deploy",
			GitCommit:    "1234567890",
			remoteDeploy: "true",
			expectedControl: oktetoRemoteRepoController{
				gitCommit: "1234567890",
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(constants.OktetoGitCommitEnvVar, tc.GitCommit)
			t.Setenv(constants.OKtetoDeployRemote, string(tc.remoteDeploy))
			r := NewRepository("https://my-repo/okteto/okteto")
			assert.Equal(t, "/okteto/okteto", r.url.Path)
			assert.IsType(t, tc.expectedControl, r.control)
		})
	}
}

func TestIsEqual(t *testing.T) {
	type input struct {
		r Repository
		o Repository
	}
	var tests = []struct {
		name     string
		input    input
		expected bool
	}{
		{
			name: "r is nil -> false",
			input: input{
				r: Repository{},
				o: Repository{url: &url.URL{}},
			},
			expected: false,
		},
		{
			name: "o is nil -> false",
			input: input{
				r: Repository{url: &url.URL{}},
				o: Repository{},
			},
			expected: false,
		},
		{
			name: "r and o are nil -> false",
			input: input{
				r: Repository{},
				o: Repository{},
			},
			expected: false,
		},
		{
			name: "different hostname -> false",
			input: input{
				r: Repository{url: &url.URL{Host: "my-hub"}},
				o: Repository{url: &url.URL{Host: "my-hub2"}},
			},
			expected: false,
		},
		{
			name: "different path -> false",
			input: input{
				r: Repository{url: &url.URL{Host: "my-hub", Path: "okteto/repo1"}},
				o: Repository{url: &url.URL{Host: "my-hub", Path: "okteto/repo2"}},
			},
			expected: false,
		},
		{
			name: "equal -> true",
			input: input{
				r: Repository{url: &url.URL{Host: "my-hub", Path: "okteto/repo1"}},
				o: Repository{url: &url.URL{Host: "my-hub", Path: "okteto/repo2"}},
			},
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.input.r.IsEqual(tt.input.o)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCleanPath(t *testing.T) {
	var tests = []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "path starts with /",
			input:    "/okteto/okteto",
			expected: "okteto/okteto",
		},
		{
			name:     "path ends with .git",
			input:    "okteto/okteto.git",
			expected: "okteto/okteto",
		},
		{
			name:     "path starts with / and ends with .git",
			input:    "/okteto/okteto.git",
			expected: "okteto/okteto",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanPath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
