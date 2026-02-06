package os

import (
	"fmt"
	"os/exec"

	log "github.com/sirupsen/logrus"
)

var (
	// ErrNoShell is used when there is no shell available in the $PATH
	ErrNoShell = fmt.Errorf("bash or sh needs to be available in the $PATH of your development container")
)

// GetShell returns the available shell
func GetShell() (string, error) {
	if p, err := exec.LookPath("bash"); err == nil {
		log.Printf("bash exists at %s", p)
		return "bash", nil
	}

	if p, err := exec.LookPath("sh"); err == nil {
		log.Printf("sh exists at %s", p)
		return "sh", nil
	}

	return "", ErrNoShell
}
