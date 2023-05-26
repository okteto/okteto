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
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/mock"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/stretchr/testify/assert"
)

// CommandContextMock is a mock for the exec.CommandContext.
type CommandContextMock struct {
	mock.Mock
}

func (m *CommandContextMock) Output() ([]byte, error) {
	args := m.Called()
	return args.Get(0).([]byte), args.Error(1)
}

// fixDubiousOwnershipConfigMock is a mock for the fixDubiousOwnershipConfig.
func fixDubiousOwnershipConfigMock(dirPath string) error {
	// You could add some logic here to simulate different scenarios.
	return nil
}

//func TestRunGitStatusCommand(t *testing.T) {
//	ctx := context.Background()

//// Test when fixAttempt is more than 0
//gitPath, dirPath := "/usr/bin/git", "/path/to/repo"
//output, err := runGitStatusCommand(ctx, gitPath, dirPath, 1)
//assert.Equal(t, "", output)
//assert.EqualError(t, err, "failed to get status: too many attempts")

// Test when command execution returns "detected dubious ownership in repository" error
//mockCmd := new(CommandContextMock)
//mockCmd.On("Output").Return(nil, errors.New("detected dubious ownership in repository"))
//output, err := runGitStatusCommand(ctx, gitPath, dirPath, 0)
//assert.Equal(t, "", output)
//assert.Nil(t, err)
//mockCmd.AssertCalled(t, "Output")

// Test when fixDubiousOwnershipConfig fails
// (you'd need to implement this part yourself)

//// Test when command executes without any error
//mockCmd = new(CommandContextMock)
//mockCmd.On("Output").Return([]byte("M file1.txt\n"), nil)
//output, err = runGitStatusCommand(ctx, gitPath, dirPath, 0)
//assert.Equal(t, "M file1.txt\n", output)
//assert.Nil(t, err)
//mockCmd.AssertCalled(t, "Output")
//}

//func TestIsClean_Timeout(t *testing.T) {
//	repo := Repository{
//		control: gitRepoController{
//			repoGetter: tt.config.repositoryGetter,
//		},
//	}
//	isClean, err := repo.IsClean()
//}

func TestIsClean(t *testing.T) {
	type config struct {
		repositoryGetter fakeRepositoryGetter
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
			// TODO: verify this test
			name: "dir is not a repository",
			config: config{
				repositoryGetter: fakeRepositoryGetter{
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
				repositoryGetter: fakeRepositoryGetter{
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
				repositoryGetter: fakeRepositoryGetter{
					repository: []*fakeRepository{
						{
							worktree: &fakeWorktree{
								status: oktetoGitStatus{
									status: git.Status{
										//"test-file.go": &git.FileStatus{
										//	Worktree: nil,
										//},
									},
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
				repositoryGetter: fakeRepositoryGetter{
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
				repositoryGetter: fakeRepositoryGetter{
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
	type config struct {
		repositoryGetter fakeRepositoryGetter
	}
	type expected struct {
		sha string
		err error
	}
	var tests = []struct {
		name     string
		config   config
		expected expected
	}{
		{
			name: "get sha without any problem",
			config: config{
				repositoryGetter: fakeRepositoryGetter{
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
						},
					},
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
				repositoryGetter: fakeRepositoryGetter{
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
				err: nil,
			},
		},
		{
			name: "error getting repository",
			config: config{
				repositoryGetter: fakeRepositoryGetter{
					repository: []*fakeRepository{
						//{
						//	worktree: &fakeWorktree{
						//		status: oktetoGitStatus{
						//			status: git.Status{
						//				"test-file.go": &git.FileStatus{
						//					Staging:  git.Unmodified,
						//					Worktree: git.Unmodified,
						//				},
						//			},
						//		},
						//	},
						//	head: plumbing.NewHashReference("test", plumbing.NewHash("test")),
						//},
					},
					err: []error{
						//nil,
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
				repositoryGetter: fakeRepositoryGetter{
					repository: []*fakeRepository{
						//{
						//	worktree: &fakeWorktree{
						//		status: oktetoGitStatus{
						//			status: git.Status{
						//				"test-file.go": &git.FileStatus{
						//					Staging:  git.Unmodified,
						//					Worktree: git.Unmodified,
						//				},
						//			},
						//		},
						//	},
						//	head: plumbing.NewHashReference("test", plumbing.NewHash("test")),
						//},
					},
					err: []error{
						//nil,
						assert.AnError},
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
