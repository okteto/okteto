package repository

import (
	"context"
	"errors"
	"github.com/go-git/go-git/v5"
	"os/exec"
	"strings"
)

var (
	errLocalGitCannotGetStatusDubiousOwner    = errors.New("detected dubious ownership in repository")
	errLocalGitCannotGetStatusTooManyAttempts = errors.New("failed to get status: too many attempts")
	errLocalGitCannotGetStatusCannotRecover   = errors.New("failed to get status: cannot recover")
	errLocalGitInvalidStatusOutput            = errors.New("failed to get git status: unexpected status line")
)

type CommandExecutor interface {
	RunCommand(ctx context.Context, dir string, name string, arg ...string) ([]byte, error)
	LookPath(file string) (string, error)
}

type LocalExec struct{}

func (*LocalExec) RunCommand(ctx context.Context, dir string, name string, arg ...string) ([]byte, error) {
	c := exec.CommandContext(ctx, name, arg...)
	c.Dir = dir
	return c.Output()
}

func (*LocalExec) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

type LocalGitInterface interface {
	Status(ctx context.Context, dirPath string, fixAttempt int) (git.Status, error)
	Exists() (string, error)
	FixDubiousOwnershipConfig(path string) error
	parseGitStatus(string) (git.Status, error)
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

func (lg *LocalGit) Status(ctx context.Context, dirPath string, fixAttempt int) (git.Status, error) {
	if fixAttempt > 0 {
		return git.Status{}, errLocalGitCannotGetStatusTooManyAttempts
	}

	output, err := lg.exec.RunCommand(ctx, lg.gitPath, "status", "--porcelain", "-z")
	if err != nil {
		if errors.Is(err, errLocalGitCannotGetStatusDubiousOwner) {
			err = lg.FixDubiousOwnershipConfig(dirPath)
			if err != nil {
				return git.Status{}, errLocalGitCannotGetStatusCannotRecover
			}
			fixAttempt++
			return lg.Status(ctx, dirPath, fixAttempt)
		}
	}

	status, err := lg.parseGitStatus(string(output))
	if err != nil {
		return git.Status{}, err
	}

	return status, err
}

func (lg *LocalGit) FixDubiousOwnershipConfig(path string) error {
	_, err := lg.exec.RunCommand(context.Background(), lg.gitPath, "config", "--global", "--add", "safe.directory", path)
	return err
}

func (lg *LocalGit) Exists() (string, error) {
	return lg.exec.LookPath("git")
}

func (*LocalGit) parseGitStatus(gitStatusOutput string) (git.Status, error) {
	lines := strings.Split(gitStatusOutput, "\000")
	status := make(map[string]*git.FileStatus, len(lines))

	for _, line := range lines {
		// line example values can be: "M modified-file.go", "?? new-file.go", etc
		parts := strings.SplitN(strings.TrimLeft(line, " "), " ", 2)
		if len(parts) == 2 {
			status[strings.Trim(parts[1], " ")] = &git.FileStatus{
				Staging: git.StatusCode([]byte(parts[0])[0]),
			}
		} else {
			return git.Status{}, errLocalGitInvalidStatusOutput
		}
	}

	return status, nil
}
