//go:build !windows
// +build !windows

package up

import "syscall"

func isProcessSessionLeader(pid int) (bool, error) {
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		return false, err
	}
	return pgid == pid, nil
}
