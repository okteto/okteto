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

package syncthing

import (
	"context"
	"fmt"
	"time"

	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
)

func (s *Syncthing) checkLocalAndRemoteStatus(ctx context.Context) error {
	if err := s.checkStatus(ctx, true); err != nil {
		return err
	}
	return s.checkStatus(ctx, false)
}

func (s *Syncthing) checkStatus(ctx context.Context, local bool) error {
	for _, folder := range s.Folders {
		status, err := s.GetStatus(ctx, &folder, local)
		if err != nil {
			return fmt.Errorf("error getting status from path:%s local=%t: %s", folder.LocalPath, local, err)
		}
		if status.PullErrors == 0 {
			continue
		}

		if err := s.GetFolderErrors(ctx, &folder, local); err != nil {
			return fmt.Errorf("error getting folder errors from path:%s local=%t: %s", folder.LocalPath, local, err)
		}
	}
	return nil
}

// Monitor will send a message to disconnected if remote syncthing is disconnected for more than 10 seconds.
func (s *Syncthing) Monitor(ctx context.Context, disconnect chan error) {
	ticker := time.NewTicker(20 * time.Second)
	retries := 0
	for {
		select {
		case <-ticker.C:
			err := s.checkLocalAndRemoteStatus(ctx)
			if err == nil {
				retries = 0
				continue
			}
			if retries >= 3 {
				log.Infof("syncthing not connected, sending disconnect signal: %s", err)
				disconnect <- errors.ErrLostSyncthing
				return
			}
			retries++
		case <-ctx.Done():
			return
		}
	}
}
