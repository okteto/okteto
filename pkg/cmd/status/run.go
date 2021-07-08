// Copyright 2021 The Okteto Authors
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

// Run runs the "okteto status" sequence
func Run(ctx context.Context, dev *model.Dev, sy *syncthing.Syncthing) (float64, error) {
	progressLocal, err := getCompletionProgress(ctx, sy, true)
	if err != nil {
		log.Infof("error accessing local syncthing status: %s", err)
		return 0, err
	}
	progressRemote, err := getCompletionProgress(ctx, sy, false)
	if err != nil {
		log.Infof("error accessing remote syncthing status: %s", err)
		return 0, err
	}

	return computeProgress(progressLocal, progressRemote), nil
}

func getCompletionProgress(ctx context.Context, s *syncthing.Syncthing, local bool) (float64, error) {
	device := syncthing.DefaultRemoteDeviceID
	if local {
		device = syncthing.LocalDeviceID
	}
	completion, err := s.GetCompletion(ctx, local, device)
	if err != nil {
		return 0, err
	}
	if completion.GlobalBytes == 0 {
		return 100, nil
	}
	progress := (float64(completion.GlobalBytes-completion.NeedBytes) / float64(completion.GlobalBytes)) * 100
	return progress, nil
}

func computeProgress(local, remote float64) float64 {
	if local == 100 && remote == 100 {
		return 100
	}

	if local == 100 {
		return remote
	}
	if remote == 100 {
		return local
	}
	return (local + remote) / 2
}

// Wait waits for the okteto up sequence to finish
func Wait(ctx context.Context, dev *model.Dev, okStatusList []config.UpState) error {
	spinner := utils.NewSpinner("Activating your development container...")
	spinner.Start()
	defer spinner.Stop()

	ticker := time.NewTicker(500 * time.Millisecond)
	for {
		status, err := config.GetState(dev)
		if err != nil {
			return err
		}
		if status == config.Failed {
			return fmt.Errorf("your development container has failed")
		}
		for _, okStatus := range okStatusList {
			if status == okStatus {
				return nil
			}
		}
		<-ticker.C
	}
}
