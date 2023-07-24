//go:build windows
// +build windows

package up

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
)

// GetCommandToExec implementation for windows GOOS
func (he *hybridExecutor) GetCommandToExec(ctx context.Context, cmd []string) (*exec.Cmd, error) {
	var c *exec.Cmd
	if runtime.GOOS != "windows" {
		c = exec.Command("bash", "-c", strings.Join(cmd, " "))
	} else {
		binary, err := expandExecutableInCurrentDirectory(cmd[0], he.workdir)
		if err != nil {
			return nil, err
		}
		c = exec.Command(binary, cmd[1:]...)
	}

	c.Env = he.envs

	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	c.Dir = he.workdir

	// https://docs.studygolang.com/pkg/syscall/?GOOS=windows#SysProcAttr
	c.SysProcAttr = &syscall.SysProcAttr{
		// SysProcAttr for windows are different from linux,
		// use CreationFlags to group the processes
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}

	return c, nil
}
