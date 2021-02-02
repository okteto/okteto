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
	"time"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/syncthing"
)

//Run runs the "okteto status" sequence
func Run(ctx context.Context, dev *model.Dev, sy *syncthing.Syncthing) (float64, error) {
	progressLocal, err := sy.GetCompletionProgress(ctx, true)
	if err != nil {
		log.Infof("error accessing local syncthing status: %s", err)
		return 0, fmt.Errorf("error accessing local syncthing status")
	}
	progressRemote, err := sy.GetCompletionProgress(ctx, false)
	if err != nil {
		log.Infof("error accessing remote syncthing status: %s", err)
		return 0, fmt.Errorf("error accessing remote syncthing status")
	}
	progress := (progressLocal + progressRemote) / 2
	return progress, nil
}

//Wait waits for the okteto up sequence to finish
func Wait(ctx context.Context, dev *model.Dev) error {
	spinner := utils.NewSpinner("Activating your development container...")
	spinner.Start()
	defer spinner.Stop()
	for {
		status, err := config.GetState(dev)
		if err != nil {
			return err
		}
		if status == config.Failed {
			return fmt.Errorf("your development container has failed")
		}
		if status == config.Ready {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
}
