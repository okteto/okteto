// Copyright 2023 The Okteto Authors
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
	"time"

	"github.com/okteto/okteto/cmd/utils"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/ssh"
	"github.com/okteto/okteto/pkg/syncthing"
)

func downloadSyncthing() error {
	maxRetries := 2
	t := time.NewTicker(1 * time.Second)
	var err error
	for i := 0; i < 3; i++ {
		p := &utils.ProgressBar{}
		err = syncthing.Install(p)
		if err == nil {
			return nil
		}

		if i < maxRetries {
			oktetoLog.Infof("failed to download syncthing, retrying: %s", err)
			<-t.C
		}
	}

	return err
}

func sshKeys() error {
	if !ssh.KeyExists() {
		oktetoLog.Spinner("Generating your client certificates...")
		oktetoLog.StartSpinner()
		defer oktetoLog.StopSpinner()

		if err := ssh.GenerateKeys(); err != nil {
			return err
		}

		oktetoLog.Success("Client certificates generated")
	}

	return nil
}
