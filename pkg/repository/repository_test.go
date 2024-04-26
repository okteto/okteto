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
	"net/url"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
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
	err          error
	worktree     *fakeWorktree
	head         *plumbing.Reference
	commit       string
	diff         string
	failInCommit bool
}

func (fr fakeRepository) Worktree() (gitWorktreeInterface, error) {
	return fr.worktree, fr.err
}
func (fr fakeRepository) Head() (*plumbing.Reference, error) {
	if fr.failInCommit {
		return fr.head, nil
	}
	return fr.head, fr.err
}

func (fr fakeRepository) GetLatestSHA(context.Context, string, string, LocalGitInterface) (string, error) {
	return fr.commit, fr.err
}

func (fr fakeRepository) Log(logOpts *git.LogOptions) (object.CommitIter, error) {
	return nil, nil
}

func (fr fakeRepository) GetDiff(ctx context.Context, repoPath, dirpath string, localGit LocalGitInterface) (string, error) {
	return fr.diff, fr.err
}

func (fr fakeRepository) calculateUntrackedFiles(ctx context.Context, contextDir string) ([]string, error) {
	return []string{}, fr.err
}

type fakeWorktree struct {
	err            error
	status         oktetoGitStatus
	root           string
	untrackedFiles []string
}

func (fw fakeWorktree) GetRoot() string {
	return fw.root
}

func (fw fakeWorktree) Status(context.Context, string, LocalGitInterface) (oktetoGitStatus, error) {
	return fw.status, fw.err
}

func (fw fakeWorktree) ListUntrackedFiles(context.Context, string, LocalGitInterface) ([]string, error) {
	return fw.untrackedFiles, fw.err
}

func TestNewRepo(t *testing.T) {
	tt := []struct {
		expectedControl repositoryInterface
		name            string
		GitCommit       string
		remoteDeploy    string
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
			t.Setenv(constants.OktetoDeployRemote, tc.remoteDeploy)
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
				o: Repository{url: &repositoryURL{}},
			},
			expected: false,
		},
		{
			name: "o is nil -> false",
			input: input{
				r: Repository{url: &repositoryURL{}},
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
				r: Repository{url: &repositoryURL{url.URL{Host: "my-hub"}}},
				o: Repository{url: &repositoryURL{url.URL{Host: "my-hub2"}}},
			},
			expected: false,
		},
		{
			name: "different path -> false",
			input: input{
				r: Repository{url: &repositoryURL{url.URL{Host: "my-hub", Path: "okteto/repo1"}}},
				o: Repository{url: &repositoryURL{url.URL{Host: "my-hub", Path: "okteto/repo2"}}},
			},
			expected: false,
		},
		{
			name: "equal -> true",
			input: input{
				r: Repository{url: &repositoryURL{url.URL{Host: "my-hub", Path: "okteto/repo1"}}},
				o: Repository{url: &repositoryURL{url.URL{Host: "my-hub", Path: "okteto/repo2"}}},
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

func Test_GetAnonymizedRepo(t *testing.T) {
	tests := []struct {
		name       string
		repository *Repository
		expected   string
	}{
		{
			name: "https repo without credentials",
			repository: &Repository{
				url: &repositoryURL{
					url.URL{
						Scheme: "https",
						Host:   "github.com",
						Path:   "/okteto/okteto",
					},
				},
			},
			expected: "https://github.com/okteto/okteto",
		},
		{
			name: "ssh repo",
			repository: &Repository{
				url: &repositoryURL{
					url.URL{
						Scheme: "ssh",
						Host:   "github.com",
						Path:   "okteto/okteto.git",
						User:   url.User("git"),
					},
				}},
			expected: "https://github.com/okteto/okteto",
		},
		{
			name: "https repo with credentials",
			repository: &Repository{
				url: &repositoryURL{
					url.URL{
						Scheme: "https",
						Host:   "github.com",
						Path:   "/okteto/okteto",
						User:   url.UserPassword("git", "PASSWORD"),
					},
				}},
			expected: "https://github.com/okteto/okteto",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.repository.GetAnonymizedRepo()
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestRepositoryURL_String(t *testing.T) {
	tests := []struct {
		name     string
		url      repositoryURL
		expected string
	}{
		{
			name: "http scheme",
			url: repositoryURL{
				URL: url.URL{
					Scheme: "http",
					Host:   "okteto.com",
					Path:   "docs",
					User:   url.UserPassword("test", "password"),
				},
			},
			expected: "https://okteto.com/docs",
		},
		{
			name: "https scheme",
			url: repositoryURL{
				URL: url.URL{
					Scheme: "https",
					Host:   "okteto.com",
					Path:   "docs",
					User:   url.UserPassword("test", "password"),
				},
			},
			expected: "https://okteto.com/docs",
		},
		{
			name: "ssh scheme",
			url: repositoryURL{
				URL: url.URL{
					Scheme: "ssh",
					Host:   "okteto.com",
					Path:   "docs",
					User:   url.UserPassword("test", "password"),
				},
			},
			expected: "https://okteto.com/docs",
		},
		{
			name: "git scheme",
			url: repositoryURL{
				URL: url.URL{
					Scheme: "ssh",
					Host:   "okteto.com",
					Path:   "okteto/okteto.git",
				},
			},
			expected: "https://okteto.com/okteto/okteto",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.url.String()
			assert.Equal(t, tt.expected, got)
		})
	}
}
