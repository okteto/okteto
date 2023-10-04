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

	"github.com/go-git/go-git/v5"
	"github.com/stretchr/testify/assert"
)

func TestIsCleanContext(t *testing.T) {
	type config struct {
		repositoryGetter *fakeRepositoryGetter
	}
	type expected struct {
		isClean bool
		err     error
	}
	var tests = []struct {
		name     string
		config   config
		expected expected
	}{
		{
			name: "dir is not a repository",
			config: config{
				repositoryGetter: &fakeRepositoryGetter{
					repository: nil,
					err:        []error{git.ErrRepositoryNotExists},
				},
			},
			expected: expected{
				isClean: false,
				err:     git.ErrRepositoryNotExists,
			},
		},
		{
			name: "repository could not access worktree",
			config: config{
				repositoryGetter: &fakeRepositoryGetter{
					repository: []*fakeRepository{
						{
							worktree: nil,
							err:      assert.AnError,
						},
					},
				},
			},
			expected: expected{
				isClean: false,
				err:     assert.AnError,
			},
		},
		{
			name: "worktree could not access status",
			config: config{
				repositoryGetter: &fakeRepositoryGetter{
					repository: []*fakeRepository{
						{
							worktree: &fakeWorktree{
								status: oktetoGitStatus{
									status: git.Status{},
								},
								err: assert.AnError,
							},
							err: nil,
						},
					},
				},
			},
			expected: expected{
				isClean: false,
				err:     assert.AnError,
			},
		},
		{
			name: "context is not clean",
			config: config{
				repositoryGetter: &fakeRepositoryGetter{
					repository: []*fakeRepository{
						{
							worktree: &fakeWorktree{
								status: oktetoGitStatus{
									status: git.Status{
										"test-file.go": &git.FileStatus{
											Staging:  git.Modified,
											Worktree: git.Unmodified,
										},
									},
								},
								err: nil,
							},
							err: nil,
						},
					},
				},
			},
			expected: expected{
				isClean: false,
				err:     nil,
			},
		},
		{
			name: "context is clean",
			config: config{
				repositoryGetter: &fakeRepositoryGetter{
					repository: []*fakeRepository{
						{
							worktree: &fakeWorktree{
								status: oktetoGitStatus{
									status: git.Status{
										"test-file.go": &git.FileStatus{
											Staging:  git.Unmodified,
											Worktree: git.Unmodified,
										},
										"test-file-2.go": &git.FileStatus{
											Staging:  git.Unmodified,
											Worktree: git.Unmodified,
										},
									},
								},
								err: nil,
							},
							err: nil,
						},
					},
				},
			},
			expected: expected{
				isClean: true,
				err:     nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := Repository{
				control: gitRepoController{
					repoGetter: tt.config.repositoryGetter,
				},
			}
			isClean, err := repo.IsCleanContext("")
			assert.ErrorIs(t, err, tt.expected.err)
			assert.Equal(t, tt.expected.isClean, isClean)
		})
	}
}
