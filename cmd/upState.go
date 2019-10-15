package cmd

import (
	"io/ioutil"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/log"
)

type upState string

const (
	starting      upState = "starting"
	startingSync  upState = "startingSync"
	synchronizing upState = "synchronizing"
	activating    upState = "activating"
	ready         upState = "ready"
	failed        upState = "failed"
)

func (up *UpContext) updateStateFile(state upState) {
	if len(up.Dev.Namespace) == 0 {
		log.Info("can't update state file, namespace is empty")
	}

	if len(up.Dev.Name) == 0 {
		log.Info("can't update state file, name is empty")
	}

	s := config.GetStateFile(up.Dev.Namespace, up.Dev.Name)
	log.Debugf("updating statefile %s with path %s", s, state)
	if err := ioutil.WriteFile(s, []byte(state), 0644); err != nil {
		log.Infof("can't update state file, %s", err)
	}
}
