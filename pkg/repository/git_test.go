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
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestIsClean(t *testing.T) {
	type config struct {
		repositoryGetter *fakeRepositoryGetter
	}
	type expected struct {
		err     error
		isClean bool
	}
	var tests = []struct {
		expected expected
		config   config
		name     string
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
			name: "repository is not clean",
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
			name: "repository is clean",
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
			isClean, err := repo.IsClean()
			assert.ErrorIs(t, err, tt.expected.err)
			assert.Equal(t, tt.expected.isClean, isClean)
		})
	}
}

func TestGetSHA(t *testing.T) {
	cleanRepo := &fakeRepository{worktree: &fakeWorktree{
		status: oktetoGitStatus{
			status: git.Status{
				"test-file.go": &git.FileStatus{
					Staging:  git.Unmodified,
					Worktree: git.Unmodified,
				},
			},
		},
	},
		head: plumbing.NewHashReference("test", plumbing.NewHash("test")),
		err:  nil,
	}

	repoWithHeadErr := &fakeRepository{worktree: &fakeWorktree{
		status: oktetoGitStatus{
			status: git.Status{
				"test-file.go": &git.FileStatus{
					Staging:  git.Unmodified,
					Worktree: git.Unmodified,
				},
			},
		},
	},
		head: plumbing.NewHashReference("test", plumbing.NewHash("test")),
		err:  assert.AnError,
	}

	type config struct {
		repositoryGetter *fakeRepositoryGetter
	}
	type expected struct {
		err error
		sha string
	}
	var tests = []struct {
		expected expected
		config   config
		name     string
	}{
		{
			name: "get sha without any problem",
			config: config{
				repositoryGetter: &fakeRepositoryGetter{
					repository: []*fakeRepository{cleanRepo, cleanRepo},
				},
			},
			expected: expected{
				sha: plumbing.NewHash("test").String(),
				err: nil,
			},
		},
		{
			name: "get empty sha when not clean",
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
							},
							head: plumbing.NewHashReference("test", plumbing.NewHash("test")),
						},
					},
				},
			},
			expected: expected{
				sha: "",
				err: errNotCleanRepo,
			},
		},
		{
			name: "error getting repository",
			config: config{
				repositoryGetter: &fakeRepositoryGetter{
					repository: []*fakeRepository{cleanRepo},
					err: []error{
						nil,
						assert.AnError},
				},
			},
			expected: expected{
				sha: "",
				err: assert.AnError,
			},
		},
		{
			name: "error getting Head",
			config: config{
				repositoryGetter: &fakeRepositoryGetter{
					repository: []*fakeRepository{cleanRepo, repoWithHeadErr},
					err: []error{
						nil,
						nil},
				},
			},
			expected: expected{
				sha: "",
				err: assert.AnError,
			},
		},
		{
			name: "error calling isClean",
			config: config{
				repositoryGetter: &fakeRepositoryGetter{
					repository: []*fakeRepository{},
					err:        []error{assert.AnError},
				},
			},
			expected: expected{
				sha: "",
				err: assert.AnError,
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
			sha, err := repo.GetSHA()
			assert.ErrorIs(t, err, tt.expected.err)
			assert.Equal(t, tt.expected.sha, sha)
		})
	}
}

func TestGetTreeHash(t *testing.T) {
	type config struct {
		repositoryGetter *fakeRepositoryGetter
	}
	type expected struct {
		err error
		sha string
	}
	var tests = []struct {
		expected     expected
		config       config
		name         string
		buildContext string
	}{
		{
			name: "get tree hash without any problem",
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
									},
								},
							},
							head: plumbing.NewHashReference("test", plumbing.NewHash("test")),
							commit: &fakeCommit{
								tree: &object.Tree{
									Entries: []object.TreeEntry{
										{
											Name: "test",
											Hash: plumbing.NewHash("test"),
										},
									},
								},
							},
							err: nil,
						},
					},
				},
			},
			buildContext: "test",
			expected: expected{
				sha: plumbing.NewHash("test").String(),
				err: nil,
			},
		},
		{
			name: "get tree hash with error retrieving repo",
			config: config{
				repositoryGetter: &fakeRepositoryGetter{
					err: []error{assert.AnError},
				},
			},
			buildContext: "test",
			expected: expected{
				sha: "",
				err: assert.AnError,
			},
		},
		{
			name: "get tree hash with error getting head",
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
									},
								},
							},
							head: plumbing.NewHashReference("test", plumbing.NewHash("test")),
							err:  assert.AnError,
						},
					},
				},
			},
			buildContext: "test",
			expected: expected{
				sha: "",
				err: assert.AnError,
			},
		},
		{
			name: "get tree hash with error getting commit",
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
									},
								},
							},
							head:         plumbing.NewHashReference("test", plumbing.NewHash("test")),
							failInCommit: true,
							err:          assert.AnError,
						},
					},
				},
			},
			buildContext: "test",
			expected: expected{
				sha: "",
				err: assert.AnError,
			},
		},
		{
			name: "get tree hash with error getting tree",
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
									},
								},
							},
							head: plumbing.NewHashReference("test", plumbing.NewHash("test")),
							commit: &fakeCommit{
								err: assert.AnError,
							},
						},
					},
				},
			},
			buildContext: "test",
			expected: expected{
				sha: "",
				err: assert.AnError,
			},
		},
		{
			name: "get tree hash with context == .",
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
									},
								},
							},
							head: plumbing.NewHashReference("test", plumbing.NewHash("test")),
							commit: &fakeCommit{
								tree: &object.Tree{
									Hash: plumbing.NewHash("tree"),
								},
							},
						},
					},
				},
			},
			buildContext: ".",
			expected: expected{
				sha: plumbing.NewHash("tree").String(),
				err: nil,
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
			treeHash, err := repo.GetTreeHash(tt.buildContext)
			assert.ErrorIs(t, err, tt.expected.err)
			assert.Equal(t, tt.expected.sha, treeHash)
		})
	}
}

func TestFindTopLevelGitDir(t *testing.T) {
	type input struct {
		mockFs func() afero.Fs
		cwd    string
	}

	tests := []struct {
		input        input
		expectedErr  error
		name         string
		expectedPath string
	}{
		{
			name: "not found",
			input: input{
				cwd: "/home/user/test/service",
				mockFs: func() afero.Fs {
					return afero.NewMemMapFs()
				},
			},
			expectedPath: "",
			expectedErr:  errFindingRepo,
		},
		{
			name: "invalid working dir",
			input: input{
				cwd: "@",
				mockFs: func() afero.Fs {
					return afero.NewMemMapFs()
				},
			},
			expectedPath: "",
			expectedErr:  errFindingRepo,
		},
		{
			name: "found",
			input: input{
				cwd: "/tmp/example/services/api",
				mockFs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					_, err := fs.Create("/tmp/example/.git")
					assert.NoError(t, err)
					return fs
				},
			},
			expectedPath: "/tmp/example",
			expectedErr:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := tt.input.mockFs()
			path, err := FindTopLevelGitDir(tt.input.cwd, fs)

			if tt.expectedErr != nil {
				assert.ErrorIs(t, err, tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectedPath, path)
		})
	}
}
