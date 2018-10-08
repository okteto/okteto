package ksync

import (
	"os/exec"
)

//Watch watches for changes in the local folder
func Watch() error {
	cmd := exec.Command("ksync", "watch")
	return cmd.Start()
}
