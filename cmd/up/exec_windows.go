//go:build windows
// +build windows

package up

import (
	"os"
	"os/exec"
	"syscall"
)

func (he *hybridExecutor) GetCommandToExec(cmd []string) (*exec.Cmd, error) {
	var c *exec.Cmd

	binary, err := expandExecutableInCurrentDirectory(cmd[0], he.workdir)
	if err != nil {
		return nil, err
	}
	c = exec.Command(binary, cmd[1:]...)

	c.Env = he.envs

	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	c.Dir = he.workdir

	c.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}

	return c, nil
}
