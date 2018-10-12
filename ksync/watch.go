package ksync

import (
	"os/exec"
)

//Watch watches for changes in the local folder
func Watch() error {
	cmd := exec.Command("ksync", "watch", "--daemon")
	return cmd.Start()
}
