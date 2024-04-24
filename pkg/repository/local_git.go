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
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/go-git/go-git/v5"
	oktetoLog "github.com/okteto/okteto/pkg/log"
)

var (
	errLocalGitCannotGetStatusTooManyAttempts = errors.New("failed to get status: too many attempts")
	errLocalGitCannotGetStatusCannotRecover   = errors.New("failed to get status: cannot recover")
	errLocalGitInvalidStatusOutput            = errors.New("failed to get git status: unexpected status line")
	errLocalGitCannotGetCommitTooManyAttempts = errors.New("failed to get latest dir commit: too many attempts")
)

type CommandExecutor interface {
	RunCommand(ctx context.Context, dir string, name string, arg ...string) ([]byte, error)
	RunPipeCommands(ctx context.Context, dir string, cmd1 string, cmd1Args []string, cmd2 string, cmd2Args []string) ([]byte, error)
	LookPath(file string) (string, error)
}

type LocalExec struct{}

func (le *LocalExec) RunCommand(ctx context.Context, dir string, name string, arg ...string) ([]byte, error) {
	c := le.createCommand(ctx, dir, name, arg...)

	return c.Output()
}

func (*LocalExec) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

// RunPipeCommands runs two commands in a pipeline. cmd1 | cmd2. Example:
// /usr/bin/git --no-optional-locks ls-files -s . | /usr/bin/git --no-optional-locks hash-object --stdin
func (le *LocalExec) RunPipeCommands(ctx context.Context, dir string, cmd1 string, cmd1Args []string, cmd2 string, cmd2Args []string) ([]byte, error) {
	c1 := le.createCommand(ctx, dir, cmd1, cmd1Args...)
	c2 := le.createCommand(ctx, dir, cmd2, cmd2Args...)

	var errOut bytes.Buffer
	var errOut2 bytes.Buffer

	// Create a pipe to connect the stdout of c1 to the stdin of c2
	pipeReader, pipeWriter := io.Pipe()
	c1.Stdout = pipeWriter
	c1.Stderr = &errOut
	c2.Stdin = pipeReader
	c2.Stderr = &errOut2

	// Start both commands
	if err := c1.Start(); err != nil {
		return []byte{}, err
	}

	var b2 bytes.Buffer
	c2.Stdout = &b2
	if err := c2.Start(); err != nil {
		return []byte{}, err
	}

	err := c1.Wait()
	pipeWriter.Close()
	if err != nil {
		oktetoLog.Infof("error executing command %q: %s", strings.Join(c1.Args, " "), errOut.String())
		return []byte{}, err
	}

	if err := c2.Wait(); err != nil {
		oktetoLog.Infof("error executing command %q: %s", strings.Join(c2.Args, " "), errOut2.String())
		return []byte{}, err
	}

	output := strings.TrimSuffix(b2.String(), "\n")
	return []byte(output), nil
}

func (*LocalExec) createCommand(ctx context.Context, dir, cmd string, args ...string) *exec.Cmd {
	c := exec.CommandContext(ctx, cmd, args...)
	c.Cancel = func() error {
		// windows: https://pkg.go.dev/os#Signal
		// Terminating the process with Signal is not implemented for windows.
		// Windows platform will only be able to kill the process
		if runtime.GOOS == "windows" {
			return c.Process.Kill()
		}

		oktetoLog.Debugf("terminating %s - %s/%s", c.String(), dir, cmd)
		if err := c.Process.Signal(syscall.SIGTERM); err != nil {
			oktetoLog.Debugf("err at signal SIGTERM: %v", err)
		}

		time.Sleep(3 * time.Second)
		if err := c.Process.Signal(syscall.Signal(0)); err != nil {
			if errors.Is(err, os.ErrProcessDone) {
				return nil
			}
			oktetoLog.Debugf("reading signal with error %v", err)
		}
		oktetoLog.Debugf("killing %s - %s/%s", c.String(), dir, cmd)
		return c.Process.Signal(syscall.SIGKILL)
	}

	c.Env = os.Environ()
	c.Dir = dir
	return c
}

type LocalGitInterface interface {
	Status(ctx context.Context, repoRoot, dirPath string, fixAttempt int) (git.Status, error)
	Exists() (string, error)
	FixDubiousOwnershipConfig(path string) error
	parseGitStatus(string) (git.Status, error)
	GetDirContentSHA(ctx context.Context, repoRoot, dirPath string, fixAttempt int) (string, error)
	Diff(ctx context.Context, repoRoot, dirPath string, fixAttempt int) (string, error)
	ListUntrackedFiles(ctx context.Context, repoRoot, workdir string, fixAttempt int) ([]string, error)
}

type LocalGit struct {
	exec    CommandExecutor
	gitPath string
}

func NewLocalGit(gitPath string, exec CommandExecutor) *LocalGit {
	return &LocalGit{
		gitPath: gitPath,
		exec:    exec,
	}
}

// Status returns the status of the repository at the given path
func (lg *LocalGit) Status(ctx context.Context, repoRoot, dirPath string, fixAttempt int) (git.Status, error) {
	if fixAttempt > 1 {
		return git.Status{}, errLocalGitCannotGetStatusTooManyAttempts
	}

	args := []string{"--no-optional-locks", "status", "--porcelain"}
	if dirPath != "" {
		args = append(args, dirPath)
	}
	output, err := lg.exec.RunCommand(ctx, repoRoot, lg.gitPath, args...)
	if err != nil {
		var exitError *exec.ExitError
		errors.As(err, &exitError)
		if exitError != nil {
			exitErr := string(exitError.Stderr)
			if strings.Contains(exitErr, "detected dubious ownership in repository") {
				err = lg.FixDubiousOwnershipConfig(repoRoot)
				if err != nil {
					return git.Status{}, errLocalGitCannotGetStatusCannotRecover
				}
				fixAttempt++
				return lg.Status(ctx, repoRoot, dirPath, fixAttempt)
			}
		}
		return git.Status{}, errLocalGitCannotGetStatusCannotRecover
	}

	status, err := lg.parseGitStatus(string(output))
	if err != nil {
		return git.Status{}, err
	}

	return status, err
}

// FixDubiousOwnershipConfig adds the given path to the git config safe.directory to avoid the dubious ownership error
func (lg *LocalGit) FixDubiousOwnershipConfig(path string) error {
	_, err := lg.exec.RunCommand(context.Background(), path, lg.gitPath, "config", "--global", "--add", "safe.directory", path)
	return err
}

// Exists checks if git binary exists in the system
func (lg *LocalGit) Exists() (string, error) {
	var err error
	lg.gitPath, err = lg.exec.LookPath("git")
	return lg.gitPath, err
}

func (*LocalGit) parseGitStatus(gitStatusOutput string) (git.Status, error) {
	lines := strings.Split(gitStatusOutput, "\n")
	status := make(map[string]*git.FileStatus, len(lines))

	for _, line := range lines {
		if line == "" {
			continue
		}
		maxValidGitStatusParts := 2
		// line example values can be: "M modified-file.go", "?? new-file.go", etc
		parts := strings.SplitN(strings.TrimLeft(line, " "), " ", maxValidGitStatusParts)
		if len(parts) == maxValidGitStatusParts {
			status[strings.Trim(parts[1], " ")] = &git.FileStatus{
				Staging: git.StatusCode([]byte(parts[0])[0]),
			}
		} else {
			return git.Status{}, errLocalGitInvalidStatusOutput
		}
	}

	return status, nil
}

func (lg *LocalGit) ListUntrackedFiles(ctx context.Context, repoRoot, workdir string, fixAttempt int) ([]string, error) {
	if fixAttempt > 1 {
		return []string{}, errLocalGitCannotGetCommitTooManyAttempts
	}

	lsFilesCmdArgs := []string{"--no-optional-locks", "ls-files", "--others", "--exclude-standard", workdir}

	output, err := lg.exec.RunCommand(ctx, repoRoot, lg.gitPath, lsFilesCmdArgs...)
	if err != nil {
		var exitError *exec.ExitError
		errors.As(err, &exitError)
		if exitError != nil {
			exitErr := string(exitError.Stderr)
			if strings.Contains(exitErr, "detected dubious ownership in repository") {
				err = lg.FixDubiousOwnershipConfig(repoRoot)
				if err != nil {
					return []string{}, errLocalGitCannotGetStatusCannotRecover
				}
				fixAttempt++
				return lg.ListUntrackedFiles(ctx, repoRoot, workdir, fixAttempt)
			}
		}
		return []string{}, errLocalGitCannotGetStatusCannotRecover
	}

	lines := strings.Split(string(output), "\n")
	untrackedFiles := []string{}
	for _, line := range lines {
		if line == "" {
			continue
		}
		untrackedFiles = append(untrackedFiles, line)
	}
	// We need to sort the untracked files to make deterministic the order of the files
	sort.Strings(untrackedFiles)
	return untrackedFiles, nil
}

// GetDirContentSHA calculates the SHA of the content of the given directory using git ls-files and git hash-object
// commands
func (lg *LocalGit) GetDirContentSHA(ctx context.Context, gitPath, dirPath string, fixAttempt int) (string, error) {
	if fixAttempt > 1 {
		return "", errLocalGitCannotGetCommitTooManyAttempts
	}

	lsFilesCmdArgs := []string{"--no-optional-locks", "ls-files", "-s", dirPath}
	hashObjectCmdArgs := []string{"--no-optional-locks", "hash-object", "--stdin"}

	output, err := lg.exec.RunPipeCommands(ctx, gitPath, lg.gitPath, lsFilesCmdArgs, lg.gitPath, hashObjectCmdArgs)
	if err != nil {
		var exitError *exec.ExitError
		errors.As(err, &exitError)
		if exitError != nil {
			exitErr := string(exitError.Stderr)
			if strings.Contains(exitErr, "detected dubious ownership in repository") {
				err = lg.FixDubiousOwnershipConfig(gitPath)
				if err != nil {
					return "", errLocalGitCannotGetStatusCannotRecover
				}
				fixAttempt++
				return lg.GetDirContentSHA(ctx, gitPath, dirPath, fixAttempt)
			}
		}
		return "", errLocalGitCannotGetStatusCannotRecover
	}
	return string(output), nil
}

// Diff returns the diff of the repository at the given path
func (lg *LocalGit) Diff(ctx context.Context, gitPath, dirPath string, fixAttempt int) (string, error) {
	if fixAttempt > 1 {
		return "", errLocalGitCannotGetCommitTooManyAttempts
	}

	output, err := lg.exec.RunCommand(ctx, gitPath, lg.gitPath, "--no-optional-locks", "diff", "--no-color", "--", "HEAD", dirPath)
	if err != nil {
		var exitError *exec.ExitError
		errors.As(err, &exitError)
		if exitError != nil {
			exitErr := string(exitError.Stderr)
			if strings.Contains(exitErr, "detected dubious ownership in repository") {
				err = lg.FixDubiousOwnershipConfig(gitPath)
				if err != nil {
					return "", errLocalGitCannotGetStatusCannotRecover
				}
				fixAttempt++
				return lg.GetDirContentSHA(ctx, gitPath, dirPath, fixAttempt)
			}
		}
		return "", errLocalGitCannotGetStatusCannotRecover
	}
	return string(output), nil
}
