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

package syncthing

import (
	"context"
	"time"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
)

const (
	completedProgress     = 100
	maxNeedDeletesRetries = 50
)

// Completion represents the completion of a syncthing folder.
type Completion struct {
	Completion  float64 `json:"completion"`
	GlobalBytes int64   `json:"globalBytes"`
	NeedBytes   int64   `json:"needBytes"`
	GlobalItems int64   `json:"globalItems"`
	NeedItems   int64   `json:"needItems"`
	NeedDeletes int64   `json:"needDeletes"`
}

// waitForCompletion represents a wait for completion iteration
type waitForCompletion struct {
	localCompletion           *Completion
	remoteCompletion          *Completion
	sy                        *Syncthing
	previousLocalGlobalBytes  int64
	previousRemoteGlobalBytes int64
	globalBytesRetries        int64
	needDeletesRetries        int64
	retries                   int64
	progress                  float64
}

// WaitForCompletion waits for the remote to be totally synched
func (s *Syncthing) WaitForCompletion(ctx context.Context, reporter chan float64) error {
	defer close(reporter)
	ticker := time.NewTicker(250 * time.Millisecond)
	wfc := &waitForCompletion{sy: s}
	for {
		select {
		case <-ticker.C:
			wfc.retries++
			if wfc.retries%40 == 0 {
				oktetoLog.Info("checking syncthing for error....")
				if err := s.IsHealthy(ctx, false, maxRetries); err != nil {
					return err
				}
			}

			if err := s.Overwrite(ctx); err != nil {
				if err != oktetoErrors.ErrBusySyncthing {
					return err
				}
			}
			if err := wfc.computeProgress(ctx); err != nil {
				if err == oktetoErrors.ErrBusySyncthing {
					reporter <- wfc.progress
					continue
				}
				return err
			}

			reporter <- wfc.progress

			if wfc.needsDatabaseReset() {
				return oktetoErrors.ErrNeedsResetSyncError
			}

			if wfc.isCompleted() {
				return nil
			}

		case <-ctx.Done():
			oktetoLog.Info("call to syncthing.WaitForCompletion canceled")
			return ctx.Err()
		}
	}
}

func (wfc *waitForCompletion) computeProgress(ctx context.Context) error {
	localCompletion, err := wfc.sy.GetCompletion(ctx, true, DefaultRemoteDeviceID)
	if err != nil {
		return err
	}
	wfc.localCompletion = localCompletion
	oktetoLog.Infof("syncthing status in local: globalBytes %d, needBytes %d, globalItems %d, needItems %d, needDeletes %d", localCompletion.GlobalBytes, localCompletion.NeedBytes, localCompletion.GlobalItems, localCompletion.NeedItems, localCompletion.NeedDeletes)
	if localCompletion.GlobalBytes == 0 {
		wfc.progress = completedProgress
	} else {
		wfc.progress = (float64(localCompletion.GlobalBytes-localCompletion.NeedBytes) / float64(localCompletion.GlobalBytes)) * 100
	}

	remoteCompletion, err := wfc.sy.GetCompletion(ctx, false, DefaultRemoteDeviceID)
	if err != nil {
		return err
	}
	wfc.remoteCompletion = remoteCompletion
	oktetoLog.Infof("syncthing status in remote: globalBytes %d, needBytes %d, globalItems %d, needItems %d, needDeletes %d",
		remoteCompletion.GlobalBytes,
		remoteCompletion.NeedBytes,
		remoteCompletion.GlobalItems,
		remoteCompletion.NeedItems,
		remoteCompletion.NeedDeletes,
	)
	return nil
}

func (wfc *waitForCompletion) needsDatabaseReset() bool {
	if wfc.localCompletion.GlobalBytes == wfc.remoteCompletion.GlobalBytes {
		wfc.globalBytesRetries = 0
		wfc.previousLocalGlobalBytes = wfc.localCompletion.GlobalBytes
		wfc.previousRemoteGlobalBytes = wfc.remoteCompletion.GlobalBytes
		return false
	}
	oktetoLog.Infof("local globalBytes %d, remote global bytes %d", wfc.localCompletion.GlobalBytes, wfc.remoteCompletion.GlobalBytes)
	if wfc.localCompletion.GlobalBytes != wfc.previousLocalGlobalBytes {
		oktetoLog.Infof("local globalBytes has changed %d vs %d", wfc.localCompletion.GlobalBytes, wfc.previousLocalGlobalBytes)
		wfc.previousLocalGlobalBytes = wfc.localCompletion.GlobalBytes
		wfc.previousRemoteGlobalBytes = wfc.remoteCompletion.GlobalBytes
		wfc.globalBytesRetries = 0
		return false
	}
	if wfc.remoteCompletion.GlobalBytes != wfc.previousRemoteGlobalBytes {
		oktetoLog.Infof("remote globalBytes has changed %d vs %d", wfc.remoteCompletion.GlobalBytes, wfc.previousRemoteGlobalBytes)
		wfc.previousLocalGlobalBytes = wfc.localCompletion.GlobalBytes
		wfc.previousRemoteGlobalBytes = wfc.remoteCompletion.GlobalBytes
		wfc.globalBytesRetries = 0
		return false
	}
	wfc.globalBytesRetries++
	oktetoLog.Infof("globalBytesRetries %d", wfc.globalBytesRetries)
	return wfc.globalBytesRetries > 360 // 90 seconds
}

func (wfc *waitForCompletion) isCompleted() bool {
	if wfc.localCompletion.NeedBytes != wfc.remoteCompletion.NeedBytes {
		return false
	}
	if wfc.localCompletion.NeedBytes > 0 {
		return false
	}
	if wfc.localCompletion.GlobalBytes != wfc.remoteCompletion.GlobalBytes {
		return false
	}

	if wfc.localCompletion.NeedDeletes > 0 {
		wfc.needDeletesRetries++
		if wfc.needDeletesRetries < maxNeedDeletesRetries {
			oktetoLog.Info("synced completed, but need deletes, retrying...")
			return false
		}
	}
	if !wfc.sy.IsAllOverwritten() {
		oktetoLog.Info("synced completed, but overwrites not sent, retrying...")
		return false
	}
	return true
}
