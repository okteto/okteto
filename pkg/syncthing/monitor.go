package syncthing

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cloudnativedevelopment/cnd/pkg/log"
)

var consecutiveErrors = 1

const maxConsecutiveErrors = 3

func (s *Syncthing) isConnectedToRemote() bool {
	body, err := s.GetFromAPI("rest/system/connections")
	if err != nil {
		log.Infof("error when getting connections from the api: %s", err)
		return false
	}

	var conns syncthingConnections
	if err := json.Unmarshal(body, &conns); err != nil {
		log.Infof("error when unmarshalling response: %s", err)
		return false
	}

	if val, ok := conns.Connections[s.RemoteDeviceID]; ok {
		return val.Connected
	}

	log.Infof("RemoteDeviceID %s missing from the response", s.RemoteDeviceID)
	return false
}

//Monitor verifies that syncthing is not in a disconnected state. If so, it sends a message to the
// disconnected channel and exits
func (s *Syncthing) Monitor(ctx context.Context, disconnected chan struct{}) {
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-ticker.C:
			if !s.isConnectedToRemote() {
				log.Debugf("not connected to syncthing, try %d/%d", consecutiveErrors, maxConsecutiveErrors)
				consecutiveErrors++
				if consecutiveErrors > maxConsecutiveErrors {
					log.Infof("not connected to syncthing, sending disconnect notification")
					disconnected <- struct{}{}
					return
				}
			} else {
				consecutiveErrors = 1
			}

		case <-ctx.Done():
			return
		}
	}
}
