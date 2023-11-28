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

package status

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/syncthing"
)

const (
	completedProgressValue = 100
)

// Run runs the "okteto status" sequence
func Run(ctx context.Context, sy *syncthing.Syncthing) (float64, error) {
	progressLocal, err := getCompletionProgress(ctx, sy, true)
	if err != nil {
		oktetoLog.Infof("error accessing local syncthing status: %s", err)
		return 0, err
	}
	progressRemote, err := getCompletionProgress(ctx, sy, false)
	if err != nil {
		oktetoLog.Infof("error accessing remote syncthing status: %s", err)
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
		return completedProgressValue, nil
	}
	progress := (float64(completion.GlobalBytes-completion.NeedBytes) / float64(completion.GlobalBytes)) * 100
	return progress, nil
}

func computeProgress(local, remote float64) float64 {
	if local == completedProgressValue && remote == completedProgressValue {
		return completedProgressValue
	}

	if local == completedProgressValue {
		return remote
	}
	if remote == completedProgressValue {
		return local
	}
	return (local + remote) / 2
}

// Wait waits for the okteto up sequence to finish
func Wait(dev *model.Dev, okStatusList []config.UpState) error {
	oktetoLog.Spinner("Activating your development container...")
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)

	go func() {

		ticker := time.NewTicker(500 * time.Millisecond)
		for {
			status, err := config.GetState(dev.Name, dev.Namespace)
			if err != nil {
				exit <- err
				return
			}
			if status == config.Failed {
				exit <- fmt.Errorf("your development container has failed")
				return
			}
			for _, okStatus := range okStatusList {
				if status == okStatus {
					exit <- nil
					return
				}
			}
			<-ticker.C
		}
	}()

	select {
	case <-stop:
		oktetoLog.Infof("CTRL+C received, starting shutdown sequence")
		oktetoLog.StopSpinner()
		return oktetoErrors.ErrIntSig
	case err := <-exit:
		if err != nil {
			oktetoLog.Infof("exit signal received due to error: %s", err)
			return err
		}
	}
	return nil
}
