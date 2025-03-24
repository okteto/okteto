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

	"github.com/okteto/okteto/pkg/ignore"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockLocalGit struct {
	mock.Mock
}

func (mlg *mockLocalGit) RunCommand(ctx context.Context, dir string, name string, arg ...string) ([]byte, error) {
	args := mlg.Called(ctx, dir, name, arg)
	return args.Get(0).([]byte), args.Error(1)
}

func (mlg *mockLocalGit) LookPath(file string) (string, error) {
	args := mlg.Called(file)
	return args.String(0), args.Error(1)
}

func (mlg *mockLocalGit) RunPipeCommands(ctx context.Context, dir string, cmd1 string, cmd1Args []string, cmd2 string, cmd2Args []string) ([]byte, error) {
	args := mlg.Called(ctx, dir, cmd1, cmd1Args, cmd2, cmd2Args)

	return args.Get(0).([]byte), args.Error(1)
}

type fakeIgnorer struct{}

func (fi *fakeIgnorer) Ignore(path string) (bool, error) {
	return path == "git/README.md", nil
}

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
			lg := NewLocalGit("git", tt.mockExec(), nil, false)
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
			lg := NewLocalGit("git", tt.mockExec(), nil, false)
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
			lg := NewLocalGit("git", tt.mock(), nil, false)
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

func TestLocalGit_GetDirContentSHAWithError(t *testing.T) {
	tests := []struct {
		expectedErr error
		mock        func() *mockLocalExec
		name        string
		fixAttempts int
	}{
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
					pipeCommand: func(ctx context.Context, dir string, cmd1 string, cmd1Args []string, cmd2 string, cmd2Args []string) ([]byte, error) {
						return nil, assert.AnError
					},
				}
			},
			expectedErr: errLocalGitCannotGetStatusCannotRecover,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lg := NewLocalGit("git", tt.mock(), nil, false)
			_, err := lg.GetDirContentSHA(context.Background(), "", "/test/dir", tt.fixAttempts)

			assert.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func TestLocalGit_GetDirContentSHAWithoutIgnore(t *testing.T) {
	dirPath := "/test/dir"
	gitPath := "git"
	lsFilesCmdArgs := []string{"--no-optional-locks", "ls-files", "-s", dirPath}
	hashObjectCmdArgs := []string{"--no-optional-locks", "hash-object", "--stdin"}
	localGit := &mockLocalGit{}

	localGit.On("RunPipeCommands", mock.Anything, gitPath, gitPath, lsFilesCmdArgs, gitPath, hashObjectCmdArgs).Return([]byte("very complex SHA"), nil)

	lg := NewLocalGit(gitPath, localGit, nil, false)

	output, err := lg.GetDirContentSHA(context.Background(), gitPath, dirPath, 0)

	localGit.AssertExpectations(t)
	require.NoError(t, err)
	require.Equal(t, "very complex SHA", output)

}

func TestLocalGit_GetDirContentSHAWithIgnore(t *testing.T) {
	dirPath := "/test/dir"
	gitPath := "git"
	localGit := &mockLocalGit{}
	lsFilesOutput := `100644 e69de29bb2d1d6434b8b29ae775ad8c2e48c5391 0	README.md
100644 f8d0d1b317c27e75dc328e1e33d2ac8ed257db44 0	main.go
100644 a45c22d80f57524e6b605e842a48bde9c455c8f0 0	pkg/utils/helpers.go
100644 2c9be049f5eb50b3c2d03de362e8f4d3e0b96fb4 0	cmd/root.go
100644 8f7e0670619e3f6d83731c99df68307d5273b82c 0	.gitignore
100755 f8d0d1b317c27e75dc328e1e33d2ac8ed257db44 0	scripts/build.sh`
	ignorer := func(filename string) ignore.Ignorer {
		return &fakeIgnorer{}
	}

	localGit.On("RunCommand", mock.Anything, dirPath, gitPath, []string{"--no-optional-locks", "ls-files", "-s"}).Return([]byte(lsFilesOutput), nil)

	localGit.On("RunCommand", mock.Anything, dirPath, gitPath, mock.Anything).Return([]byte("very complex SHA with ignorer"), nil)

	lg := NewLocalGit(gitPath, localGit, ignorer, true)
	lg.fs = afero.NewMemMapFs()

	output, err := lg.GetDirContentSHA(context.Background(), gitPath, dirPath, 0)

	localGit.AssertExpectations(t)
	require.NoError(t, err)
	require.Equal(t, "very complex SHA with ignorer", output)

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
			lg := NewLocalGit("git", tt.execMock(), nil, false)
			files, err := lg.ListUntrackedFiles(context.Background(), "/test/dir", "/test", tt.fixAttempt)

			assert.Equal(t, tt.expectedFiles, files)
			assert.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func TestLocalGit_GetDiffWithError(t *testing.T) {
	tests := []struct {
		expectedErr error
		mock        func() *mockLocalExec
		name        string
		fixAttempts int
	}{
		{
			name:        "failure due to too many attempts",
			fixAttempts: 2,
			mock: func() *mockLocalExec {
				return &mockLocalExec{
					runCommand: func(_ context.Context, _ string, _ string, _ ...string) ([]byte, error) {
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
					runCommand: func(_ context.Context, _ string, _ string, _ ...string) ([]byte, error) {
						return nil, assert.AnError
					},
				}
			},
			expectedErr: errLocalGitCannotGetStatusCannotRecover,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lg := NewLocalGit("git", tt.mock(), nil, false)
			_, err := lg.Diff(context.Background(), "", "/test/dir", tt.fixAttempts)

			assert.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func TestLocalGit_GetDiffWithoutIgnore(t *testing.T) {
	dirPath := "/test/dir"
	gitPath := "git"
	diffCmdArgs := []string{"--no-optional-locks", "diff", "--no-color", "--", "HEAD", dirPath}
	localGit := &mockLocalGit{}

	localGit.On("RunCommand", mock.Anything, gitPath, gitPath, diffCmdArgs).Return([]byte("very complex diff"), nil)

	lg := NewLocalGit(gitPath, localGit, nil, false)

	output, err := lg.Diff(context.Background(), gitPath, dirPath, 0)

	localGit.AssertExpectations(t)
	require.NoError(t, err)
	require.Equal(t, "very complex diff", output)
}

func TestLocalGit_GetDiffWithIgnore(t *testing.T) {
	dirPath := "/test/dir"
	gitPath := "git"
	diffCmdArgs := []string{"--no-optional-locks", "diff", "--no-color", "HEAD", "."}
	diffOutput := `diff --git a/README.md b/README.md
index d14a7f3..3c9e1a0 100644
--- a/README.md
+++ b/README.md
@@ -1,5 +1,11 @@
-# My Project
-This project does something useful.
-
-## Getting Started
-To get started, run "main.go".
+# My Awesome Project
+
+This project does something incredibly useful and efficient.
+
+## Requirements
+
+- Go 1.20 or higher
+- Git
+
+## Getting Started
+Run the application using: "go run main.go"

diff --git a/main.go b/main.go
index e69de29..b6fc4c3 100644
--- a/main.go
+++ b/main.go
@@ -0,0 +1,15 @@
+package main
+
+import (
+    "fmt"
+    "os"
+)
+
+func main() {
+    name := "World"
+    if len(os.Args) > 1 {
+        name = os.Args[1]
+    }
+    fmt.Printf("Hello, %s!\n", name)
+}
+`
	expectedOutput := `diff --git a/main.go b/main.go
index e69de29..b6fc4c3 100644
--- a/main.go
+++ b/main.go
@@ -0,0 +1,15 @@
+package main
+
+import (
+    "fmt"
+    "os"
+)
+
+func main() {
+    name := "World"
+    if len(os.Args) > 1 {
+        name = os.Args[1]
+    }
+    fmt.Printf("Hello, %s!\n", name)
+}
+
\ No newline at end of file
`
	localGit := &mockLocalGit{}

	localGit.On("RunCommand", mock.Anything, dirPath, gitPath, diffCmdArgs).Return([]byte(diffOutput), nil)

	ignorer := func(filename string) ignore.Ignorer {
		return &fakeIgnorer{}
	}
	lg := NewLocalGit(gitPath, localGit, ignorer, true)

	output, err := lg.Diff(context.Background(), gitPath, dirPath, 0)

	localGit.AssertExpectations(t)
	require.NoError(t, err)
	require.Equal(t, expectedOutput, output)
}
