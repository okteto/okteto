package syncthing

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cloudnativedevelopment/cnd/pkg/log"
)

var consecutiveErrors = 1
var backoffPolicy = []time.Duration{5 * time.Second, 10 * time.Second, 10 * time.Second, 20 * time.Second, 30 * time.Second}

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
		if !val.Connected {
			log.Debugf("remote device looks disconnected: %+v", conns)
		}

		return val.Connected
	}

	log.Infof("RemoteDeviceID %s missing from the response", s.RemoteDeviceID)
	return false
}

//Monitor verifies that syncthing is not in a disconnected state. If so, it sends a message to the
// disconnected channel if available.
func (s *Syncthing) Monitor(ctx context.Context, disconnected chan struct{}) {
	ticker := time.NewTicker(backoffPolicy[consecutiveErrors])
	var reconnectionTime time.Time
	for {
		select {
		case <-ticker.C:
			if !s.isConnectedToRemote() {
				if consecutiveErrors == 1 {
					reconnectionTime = time.Now()
				}

				log.Debugf("not connected to syncthing, try %d/%d", consecutiveErrors, maxConsecutiveErrors)
				consecutiveErrors++
				if consecutiveErrors > maxConsecutiveErrors {
					log.Infof("not connected to syncthing for %s seconds, sending disconnect notification", time.Now().Sub(reconnectionTime))
					if disconnected != nil {
						disconnected <- struct{}{}
						consecutiveErrors = 1
					}
				}
			} else {
				if consecutiveErrors > 1 {
					log.Infof("successfully connected to syncthing, took %s", time.Now().Sub(reconnectionTime))
				}

				consecutiveErrors = 1
			}

		case <-ctx.Done():
			return
		}

		ticker = time.NewTicker(backoffPolicy[consecutiveErrors])
	}
}
