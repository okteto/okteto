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
	"encoding/json"
	"time"

	"github.com/okteto/okteto/pkg/log"
)

// ConnectionStatus represents the status of a syncthing connections
type ConnectionStatus struct {
	Connections map[string]Connection `json:"connections"`
}

// Connection represents the status of a syncthing connection
type Connection struct {
	Connected bool `json:"connected"`
}

// isConnected returns true if it can ping the remote syncthing
func (s *Syncthing) isConnected(ctx context.Context) bool {
	var status ConnectionStatus
	body, err := s.APICall(ctx, "rest/system/connections", "GET", 200, nil, false, nil)
	if err != nil {
		log.Infof("syncthing 'rest/system/connections' failed: %s", err)
		return false
	}
	err = json.Unmarshal(body, &status)
	if err != nil {
		log.Infof("syncthing connections unmarshalling failed: %s", err)
		return false
	}
	if status.Connections == nil {
		return false
	}
	return status.Connections[localDeviceID].Connected
}

// Monitor will send a message to disconnected if remote syncthing is disconnected for more than 10 seconds.
func (s *Syncthing) Monitor(ctx context.Context, disconnect chan struct{}) {
	ticker := time.NewTicker(5 * time.Second)
	connected := true
	for {
		select {
		case <-ticker.C:
			if s.isConnected(ctx) {
				connected = true
			} else {
				if !connected {
					log.Info("not connected to syncthing, sending disconnect signal")
					disconnect <- struct{}{}
					return
				}
				connected = false
			}
		case <-ctx.Done():
			return
		}
	}
}
