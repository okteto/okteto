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

package status

import (
	"context"
	"fmt"

	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/syncthing"
)

//Run runs the "okteto status" sequence
func Run(dev *model.Dev, showInfo bool) error {
	sy, err := syncthing.Load(dev)
	if err != nil {
		return fmt.Errorf("error accessing to syncthing info file: %s", err)
	}
	if showInfo {
		log.Information("Local syncthing url: http://%s", sy.GUIAddress)
		log.Information("Remote syncthing url: http://%s", sy.RemoteGUIAddress)
		log.Information("Syncthing username: okteto")
		log.Information("Syncthing password: %s", sy.GUIPassword)
	}
	ctx := context.Background()
	status, err := sy.GetCompletion(ctx, dev)
	if err != nil {
		return fmt.Errorf("error accessing syncthing status: %s", err)
	}
	status = 99
	if status == 100 {
		log.Success("Synchronization status: %.2f%%", status)
	} else {
		log.Yellow("Synchronization status: %.2f%%", status)
	}
	return nil
}
