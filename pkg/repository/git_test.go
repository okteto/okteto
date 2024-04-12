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
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
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

func TestGetLatestDirSHA(t *testing.T) {
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
			name: "get hash without any problem",
			config: config{
				repositoryGetter: &fakeRepositoryGetter{
					repository: []*fakeRepository{
						{
							commit: "hash",
							err:    nil,
						},
					},
				},
			},
			buildContext: "test",
			expected: expected{
				sha: "hash",
				err: nil,
			},
		},
		{
			name: "get build context hash with error retrieving repo",
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
			name: "get build context hash with error getting head",
			config: config{
				repositoryGetter: &fakeRepositoryGetter{
					repository: []*fakeRepository{
						{
							commit: "",
							err:    assert.AnError,
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := Repository{
				control: gitRepoController{
					repoGetter: tt.config.repositoryGetter,
				},
			}
			commit, err := repo.GetLatestDirSHA(tt.buildContext)
			assert.ErrorIs(t, err, tt.expected.err)
			assert.Equal(t, tt.expected.sha, commit)
		})
	}
}

func TestGetDiffHash(t *testing.T) {
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
			name: "get hash without any problem",
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
							commit: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
							err:    nil,
						},
					},
				},
			},
			buildContext: "test",
			expected: expected{
				sha: "3973e022e93220f9212c18d0d0c543ae7c309e46640da93a4a0314de999f5112",
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
			commit, err := repo.GetDiffHash(tt.buildContext)
			assert.ErrorIs(t, err, tt.expected.err)
			assert.Equal(t, tt.expected.sha, commit)
		})
	}
}

func TestFindTopLevelGitDir(t *testing.T) {
	type input struct {
		mockFs func() afero.Fs
		cwd    string
	}

	rootDir, err := filepath.Abs(filepath.Clean("/tmp"))
	assert.NoError(t, err)

	tests := []struct {
		input        input
		expectedErr  error
		name         string
		expectedPath string
	}{
		{
			name: "not found",
			input: input{
				cwd: filepath.Join(rootDir, "example", "services", "api"),
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
				cwd: filepath.Join(rootDir, "example", "services", "api"),
				mockFs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					gitDirPath := filepath.Join(rootDir, "example", ".git")
					_, err := fs.Create(gitDirPath)
					assert.NoError(t, err)
					return fs
				},
			},
			expectedPath: filepath.Join(rootDir, "example"),
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

func TestGetUntrackedContent(t *testing.T) {
	type input struct {
		mockFs func() afero.Fs
		files  []string
	}

	type output struct {
		expectedErr      error
		untrackedContent string
	}

	testFile1 := filepath.Clean("/tmp/example/services/api/test.go")
	testFile2 := filepath.Clean("/tmp/example/services/api/test2.go")

	tests := []struct {
		output output
		name   string
		input  input
	}{
		{
			name: "no untracked files",
			input: input{
				files: []string{},
				mockFs: func() afero.Fs {
					return afero.NewMemMapFs()
				},
			},
			output: output{
				expectedErr:      nil,
				untrackedContent: "",
			},
		},
		{
			name: "not found file",
			input: input{
				files: []string{
					testFile1,
				},
				mockFs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					return fs
				},
			},
			output: output{
				expectedErr:      os.ErrNotExist,
				untrackedContent: "",
			},
		},
		{
			name: "one file",
			input: input{
				files: []string{
					testFile1,
				},
				mockFs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					err := afero.WriteFile(fs, testFile1, []byte("test"), 0644)
					if err != nil {
						t.Fatal(err)
					}
					return fs
				},
			},
			output: output{
				expectedErr:      nil,
				untrackedContent: fmt.Sprintf("%s:test\n", testFile1),
			},
		},
		{
			name: "more than one file",
			input: input{
				files: []string{
					testFile1,
					testFile2,
				},
				mockFs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					err := afero.WriteFile(fs, testFile1, []byte("test"), 0644)
					if err != nil {
						t.Fatal(err)
					}
					err = afero.WriteFile(fs, testFile2, []byte("test2"), 0644)
					if err != nil {
						t.Fatal(err)
					}
					return fs
				},
			},
			output: output{
				expectedErr:      nil,
				untrackedContent: fmt.Sprintf("%s:test\n%s:test2\n", testFile1, testFile2),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := tt.input.mockFs()
			gitRepoController := gitRepoController{
				fs: fs,
			}
			content, err := gitRepoController.getUntrackedContent(tt.input.files)
			assert.Equal(t, tt.output.untrackedContent, content)
			assert.ErrorIs(t, err, tt.output.expectedErr)
		})
	}
}

func Test_gitRepoController_sanitiseURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid URL",
			input:    "https://github.com/okteto/test.git",
			expected: "https://github.com/okteto/test.git",
		},
		{
			name:     "URL with double slashes",
			input:    "https://github.com//okteto//test.git",
			expected: "https://github.com/okteto/test.git",
		},
		{
			name:     "URL with multiple double slashes",
			input:    "https://github.com//okteto//test//.git",
			expected: "https://github.com/okteto/test/.git",
		},
		{
			name:     "URL with no double slashes",
			input:    "https://github.com/okteto/test/.git",
			expected: "https://github.com/okteto/test/.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gitRepoController{}
			result := r.sanitiseURL(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
