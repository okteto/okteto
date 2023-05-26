package repository

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
)

type CommandExecutor interface {
	RunCommand(ctx context.Context, dir string, name string, arg ...string) ([]byte, error)
	LookPath(file string) (string, error)
}

type LocalExec struct{}

func (le *LocalExec) RunCommand(ctx context.Context, dir string, name string, arg ...string) ([]byte, error) {
	c := exec.CommandContext(ctx, name, arg...)
	c.Dir = dir
	return c.Output()
}

func (le *LocalExec) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

type LocalGitInterface interface {
	Status(ctx context.Context, dirPath string, fixAttempt int) (string, error)
	Exists() (string, error)
	FixDubiousOwnershipConfig(path string) error
}

type LocalGit struct {
	gitPath string
	exec    CommandExecutor
}

func NewLocalGit(gitPath string, exec CommandExecutor) *LocalGit {
	return &LocalGit{
		gitPath: gitPath,
		exec:    exec,
	}
}

func (lg *LocalGit) Status(ctx context.Context, dirPath string, fixAttempt int) (string, error) {
	if fixAttempt > 0 {
		return "", fmt.Errorf("failed to get status: too many attempts")
	}

	output, err := lg.exec.RunCommand(ctx, lg.gitPath, "status", "--porcelain", "-z")
	if err != nil {
		if errors.Is(err, errors.New("detected dubious ownership in repository")) {
			err = lg.FixDubiousOwnershipConfig(dirPath)
			if err != nil {
				return "", fmt.Errorf("failed to get status: cannot recover")
			}
			fixAttempt++
			return lg.Status(ctx, dirPath, fixAttempt)
		}
	}

	return string(output), err
}

func (lg *LocalGit) FixDubiousOwnershipConfig(path string) error {
	_, err := lg.exec.RunCommand(context.Background(), lg.gitPath, "config", "--global", "--add", "safe.directory", path)
	return err
}

func (lg *LocalGit) Exists() (string, error) {
	return exec.LookPath("git")
}
