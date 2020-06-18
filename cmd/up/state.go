// Copyright 2020 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package up

import (
	"io/ioutil"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/log"
)

type upState string

const (
	activating    upState = "activating"
	starting      upState = "starting"
	attaching     upState = "attaching"
	pulling       upState = "pulling"
	startingSync  upState = "startingSync"
	synchronizing upState = "synchronizing"
	ready         upState = "ready"
	failed        upState = "failed"
)

func (up *upContext) updateStateFile(state upState) {
	if up.Dev.Namespace == "" {
		log.Info("can't update state file, namespace is empty")
	}

	if up.Dev.Name == "" {
		log.Info("can't update state file, name is empty")
	}

	s := config.GetStateFile(up.Dev.Namespace, up.Dev.Name)
	log.Debugf("updating statefile %s: '%s'", s, state)
	if err := ioutil.WriteFile(s, []byte(state), 0644); err != nil {
		log.Infof("can't update state file, %s", err)
	}
}
