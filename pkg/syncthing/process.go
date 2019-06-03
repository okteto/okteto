// +build !windows

package syncthing

import (
	"os"
	"strings"
)

func terminate(pid int) error {
	proc := os.Process{Pid: pid}
	if err := proc.Signal(os.Interrupt); err != nil {
		if strings.Contains(err.Error(), "process already finished") {
			return nil
		}

		return err
	}

	defer proc.Wait() // nolint: errcheck

	return nil
}
