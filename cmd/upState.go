package cmd

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/log"
)

type upState string

const (
	provisioning  upState = "provisioning"
	startingSync  upState = "startingSync"
	synchronizing upState = "synchronizing"
	activating    upState = "activating"
	ready         upState = "ready"
)

func (up *UpContext) updateStateFile(state upState) error {
	if len(up.Dev.Namespace) == 0 {
		return fmt.Errorf("namespace is empty")
	}

	if len(up.Dev.Name) == 0 {
		return fmt.Errorf("name is empty")
	}

	return ioutil.WriteFile(config.GetStateFile(up.Dev.Namespace, up.Dev.Name), []byte(state), 0644)
}

func (up *UpContext) deleteStateFile() {
	path := config.GetStateFile(up.Dev.Namespace, up.Dev.Name)
	if err := os.Remove(path); err != nil {
		log.Infof("failed to delete %s", path)
	}
}
