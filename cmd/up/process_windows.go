//go:build windows
// +build windows

package up

func isProcessSessionLeader(pid int) (bool, error) {
	return false, nil
}
