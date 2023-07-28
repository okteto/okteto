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
	runCommand func(ctx context.Context, dir string, name string, arg ...string) ([]byte, error)
	lookPath   func(file string) (string, error)
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

func TestLocalGit_Exists(t *testing.T) {
	tests := []struct {
		name     string
		mockExec func() *mockLocalExec
		err      error
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
			lg := NewLocalGit("git", tt.mockExec())
			_, err := lg.Exists()
			assert.ErrorIs(t, err, tt.err)
		})
	}
}

func TestLocalGit_FixDubiousOwnershipConfig(t *testing.T) {
	tests := []struct {
		name     string
		mockExec func() *mockLocalExec
		err      error
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
			lg := NewLocalGit("git", tt.mockExec())
			err := lg.FixDubiousOwnershipConfig("/test/dir")

			assert.ErrorIs(t, err, tt.err)
		})
	}
}

func TestLocalGit_Status(t *testing.T) {
	tests := []struct {
		name        string
		fixAttempts int
		mock        func() *mockLocalExec
		expectedErr error
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
			lg := NewLocalGit("git", tt.mock())
			_, err := lg.Status(context.Background(), "/test/dir", tt.fixAttempts)

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
