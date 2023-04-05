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
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/stretchr/testify/assert"
)

type fakeRepositoryGetter struct {
	repository *fakeRepository
	err        error
}

func (frg fakeRepositoryGetter) get(_ string) (gitRepositoryInterface, error) {
	return frg.repository, frg.err
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
	status *fakeStatus
	err    error
}

func (fw fakeWorktree) Status() (gitStatusInterface, error) {
	return fw.status, fw.err
}

type fakeStatus struct {
	isClean bool
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
