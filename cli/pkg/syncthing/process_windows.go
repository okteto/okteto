package syncthing

import (
	"github.com/mattn/psutil"
)

func terminate(pid int) error {
	return psutil.TerminateTree(pid, 0)
}
