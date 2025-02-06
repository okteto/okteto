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
	"os/exec"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type mockLocalExec struct {
	runCommand  func(ctx context.Context, dir string, name string, arg ...string) ([]byte, error)
	pipeCommand func(ctx context.Context, dir string, cmd1 string, cmd1Args []string, cmd2 string, cmd2Args []string) ([]byte, error)
	lookPath    func(file string) (string, error)
}

func (mle *mockLocalExec) RunCommand(ctx context.Context, dir string, name string, arg ...string) ([]byte, error) {
	if mle.runCommand != nil {
		return mle.runCommand(ctx, dir, name, arg...)
	}
	return nil, assert.AnError
}

func (mle *mockLocalExec) LookPath(file string) (string, error) {
	if mle.lookPath != nil {
		return mle.lookPath(file)
	}
	return "", assert.AnError
}

func (mle *mockLocalExec) RunPipeCommands(ctx context.Context, dir string, cmd1 string, cmd1Args []string, cmd2 string, cmd2Args []string) ([]byte, error) {
	if mle.pipeCommand != nil {
		return mle.pipeCommand(ctx, dir, cmd1, cmd1Args, cmd2, cmd2Args)
	}

	return nil, assert.AnError
}

func TestLocalGit_Exists(t *testing.T) {
	tests := []struct {
		err      error
		mockExec func() *mockLocalExec
		name     string
	}{
		{
			name: "git exists",
			mockExec: func() *mockLocalExec {
				return &mockLocalExec{
					lookPath: func(file string) (string, error) {
						return "/usr/bin/git", nil
					},
				}
			},
			err: nil,
		},
		{
			name: "git not found",
			mockExec: func() *mockLocalExec {
				return &mockLocalExec{
					lookPath: func(file string) (string, error) {
						return "", assert.AnError
					},
				}
			},
			err: assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lg := NewLocalGit("git", tt.mockExec(), nil)
			_, err := lg.Exists()
			assert.ErrorIs(t, err, tt.err)
		})
	}
}

func TestLocalGit_FixDubiousOwnershipConfig(t *testing.T) {
	tests := []struct {
		err      error
		mockExec func() *mockLocalExec
		name     string
	}{
		{
			name: "success",
			mockExec: func() *mockLocalExec {
				return &mockLocalExec{
					runCommand: func(_ context.Context, _ string, _ string, _ ...string) ([]byte, error) {
						return []byte(""), nil
					},
				}
			},
			err: nil,
		},
		{
			name: "failure",
			mockExec: func() *mockLocalExec {
				return &mockLocalExec{
					runCommand: func(ctx context.Context, dir string, name string, arg ...string) ([]byte, error) {
						return nil, assert.AnError
					},
				}
			},
			err: assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lg := NewLocalGit("git", tt.mockExec(), nil)
			err := lg.FixDubiousOwnershipConfig("/test/dir")

			assert.ErrorIs(t, err, tt.err)
		})
	}
}

func TestLocalGit_Status(t *testing.T) {
	tests := []struct {
		expectedErr error
		mock        func() *mockLocalExec
		name        string
		fixAttempts int
	}{
		{
			name:        "success",
			fixAttempts: 0,
			mock: func() *mockLocalExec {
				return &mockLocalExec{
					runCommand: func(ctx context.Context, dir string, name string, arg ...string) ([]byte, error) {
						return []byte("M modified-file.go"), nil
					},
				}
			},
			expectedErr: nil,
		},
		{
			name:        "fail to parse git status output",
			fixAttempts: 0,
			mock: func() *mockLocalExec {
				return &mockLocalExec{
					runCommand: func(ctx context.Context, dir string, name string, arg ...string) ([]byte, error) {
						return []byte("unexpected_output_in_git_status"), nil
					},
				}
			},
			expectedErr: errLocalGitInvalidStatusOutput,
		},
		{
			name:        "recover from dubious ownership",
			fixAttempts: 0,
			mock: func() *mockLocalExec {
				var currentFixAttempt int
				return &mockLocalExec{
					runCommand: func(ctx context.Context, dir string, name string, arg ...string) ([]byte, error) {
						if currentFixAttempt == 0 {
							currentFixAttempt++
							return nil, &exec.ExitError{
								Stderr: []byte("fatal: detected dubious ownership in repository at <path>"),
							}
						}

						return []byte(""), nil

					},
				}
			},
			expectedErr: nil,
		},
		{
			name:        "failure due to too many attempts",
			fixAttempts: 2,
			mock: func() *mockLocalExec {
				return &mockLocalExec{
					runCommand: func(ctx context.Context, dir string, name string, arg ...string) ([]byte, error) {
						return nil, assert.AnError
					},
				}
			},
			expectedErr: errLocalGitCannotGetStatusTooManyAttempts,
		},
		{
			name:        "cannot recover",
			fixAttempts: 1,
			mock: func() *mockLocalExec {
				return &mockLocalExec{
					runCommand: func(ctx context.Context, dir string, name string, arg ...string) ([]byte, error) {
						return nil, assert.AnError
					},
				}
			},
			expectedErr: errLocalGitCannotGetStatusCannotRecover,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lg := NewLocalGit("git", tt.mock(), nil)
			_, err := lg.Status(context.Background(), "/test/dir", "", tt.fixAttempts)

			assert.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func Test_LocalExec_RunCommandWithContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	go func(cancel context.CancelFunc) {
		time.Sleep(1 * time.Second)
		cancel()
	}(cancel)

	localExec := &LocalExec{}
	got, err := localExec.RunCommand(ctx, t.TempDir(), "sleep", "3600")

	if runtime.GOOS != "windows" {
		assert.EqualError(t, err, "signal: terminated")
	} else {
		assert.EqualError(t, err, "exit status 1")
	}
	assert.Equal(t, []byte(""), got)
}

func Test_LocalExec_RunCommand(t *testing.T) {
	ctx := context.Background()

	localExec := &LocalExec{}
	got, err := localExec.RunCommand(ctx, t.TempDir(), "echo", "okteto")
	assert.NoError(t, err)
	assert.Equal(t, "okteto\n", string(got))
}

func TestLocalGit_GetDirContentSHA(t *testing.T) {
	tests := []struct {
		expectedErr error
		mock        func() *mockLocalExec
		name        string
		fixAttempts int
	}{
		{
			name:        "success",
			fixAttempts: 0,
			mock: func() *mockLocalExec {
				return &mockLocalExec{
					runCommand: func(ctx context.Context, dir, name string, arg ...string) ([]byte, error) {
						return []byte(
							`100644 5cc5ccc76f0fa2674fd3b17c1b863d62eebcb853 0	Dockerfile
  100644 261eeb9e9f8b2b4b0d119366dda99c6fd7d35c64 0	LICENSE
  100644 50675ea7dda5ea4d4204468eaf121681c204a717 0	Makefile
  100644 5a48ac3289fbec053cc3016b9f1a46d7d59597d2 0	README.md`), nil
					},
					pipeCommand: func(ctx context.Context, dir string, cmd1 string, cmd1Args []string, cmd2 string, cmd2Args []string) ([]byte, error) {
						return []byte("123123"), nil
					},
				}
			},
			expectedErr: nil,
		},
		{
			name:        "failure due to too many attempts",
			fixAttempts: 2,
			mock: func() *mockLocalExec {
				return &mockLocalExec{
					pipeCommand: func(ctx context.Context, dir string, cmd1 string, cmd1Args []string, cmd2 string, cmd2Args []string) ([]byte, error) {
						return nil, assert.AnError
					},
				}
			},
			expectedErr: errLocalGitCannotGetCommitTooManyAttempts,
		},
		{
			name:        "cannot recover",
			fixAttempts: 1,
			mock: func() *mockLocalExec {
				return &mockLocalExec{
					runCommand: func(ctx context.Context, dir string, name string, arg ...string) ([]byte, error) {
						return nil, assert.AnError
					},
				}
			},
			expectedErr: errLocalGitCannotGetStatusCannotRecover,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lg := NewLocalGit("git", tt.mock(), nil)
			_, err := lg.GetDirContentSHA(context.Background(), "", "/test/dir", tt.fixAttempts)

			assert.ErrorIs(t, err, tt.expectedErr)
		})
	}
}
func TestLocalGit_ListUntrackedFiles(t *testing.T) {
	tests := []struct {
		expectedErr   error
		execMock      func() *mockLocalExec
		name          string
		expectedFiles []string
		fixAttempt    int
	}{
		{
			name:       "success",
			fixAttempt: 0,
			execMock: func() *mockLocalExec {
				return &mockLocalExec{
					runCommand: func(ctx context.Context, dir string, name string, arg ...string) ([]byte, error) {
						return []byte("file1.txt\nfile2.txt\nfile3.txt"), nil
					},
				}
			},
			expectedFiles: []string{"file1.txt", "file2.txt", "file3.txt"},
			expectedErr:   nil,
		},
		{
			name:       "failure - exit error",
			fixAttempt: 0,
			execMock: func() *mockLocalExec {
				return &mockLocalExec{
					runCommand: func(ctx context.Context, dir string, name string, arg ...string) ([]byte, error) {
						return nil, &exec.ExitError{
							Stderr: []byte("fatal: detected dubious ownership in repository at <path>"),
						}
					},
				}
			},
			expectedFiles: []string{},
			expectedErr:   errLocalGitCannotGetStatusCannotRecover,
		},
		{
			name:       "failure - fix attempt limit reached",
			fixAttempt: 2,
			execMock: func() *mockLocalExec {
				return &mockLocalExec{
					runCommand: func(ctx context.Context, dir string, name string, arg ...string) ([]byte, error) {
						return nil, &exec.ExitError{
							Stderr: []byte("fatal: detected dubious ownership in repository at <path>"),
						}
					},
				}
			},
			expectedFiles: []string{},
			expectedErr:   errLocalGitCannotGetCommitTooManyAttempts,
		},
		{
			name:       "failure - cannot recover",
			fixAttempt: 1,
			execMock: func() *mockLocalExec {
				return &mockLocalExec{
					runCommand: func(ctx context.Context, dir string, name string, arg ...string) ([]byte, error) {
						return nil, assert.AnError
					},
				}
			},
			expectedFiles: []string{},
			expectedErr:   errLocalGitCannotGetStatusCannotRecover,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lg := NewLocalGit("git", tt.execMock(), nil)
			files, err := lg.ListUntrackedFiles(context.Background(), "/test/dir", "/test", tt.fixAttempt)

			assert.Equal(t, tt.expectedFiles, files)
			assert.ErrorIs(t, err, tt.expectedErr)
		})
	}
}
