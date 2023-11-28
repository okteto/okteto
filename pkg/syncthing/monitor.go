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

// Monitor will send a message to disconnected if remote syncthing is disconnected for more than 10 seconds.
func (s *Syncthing) Monitor(ctx context.Context, disconnect chan error) {
	ticker := time.NewTicker(10 * time.Second)
	retries := 0
	for {
		select {
		case <-ticker.C:
			if s.checkLocalAndRemotePing(ctx) {
				retries = 0
				continue
			}
			oktetoLog.Infof("syncthing ping error %d", retries)
			if retries >= maxRetries {
				oktetoLog.Infof("syncthing ping error, sending disconnect signal")
				disconnect <- oktetoErrors.ErrLostSyncthing
				return
			}
			retries++
		case <-ctx.Done():
			return
		}
	}
}

// MonitorStatus will send a message to disconnected if there is a synchronization error
func (s *Syncthing) MonitorStatus(ctx context.Context, disconnect chan error) {
	ticker := time.NewTicker(60 * time.Second)
	for {
		select {
		case <-ticker.C:
			err := s.checkLocalAndRemoteStatus(ctx)
			switch err {
			case nil, oktetoErrors.ErrBusySyncthing, oktetoErrors.ErrLostSyncthing:
				continue
			default:
				oktetoLog.Infof("syncthing monitor error, sending disconnect signal: %s", err)
				disconnect <- err
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (s *Syncthing) checkLocalAndRemotePing(ctx context.Context) bool {
	if !s.Ping(ctx, true) {
		return false
	}
	return s.Ping(ctx, false)
}

func (s *Syncthing) checkLocalAndRemoteStatus(ctx context.Context) error {
	if err := s.IsHealthy(ctx, true, maxRetries); err != nil {
		return err
	}
	return s.IsHealthy(ctx, false, maxRetries)
}
