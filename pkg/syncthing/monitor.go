package syncthing

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cloudnativedevelopment/cnd/pkg/log"
)

// IsConnected returns true if it can talk to the syncthing API endpoint
// and the remote device looks connected
func (s *Syncthing) IsConnected() bool {
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

// Monitor will send a message to disconnected if the 'externalDevice' shows as disconnected for more than 30 seconds.
func (s *Syncthing) Monitor(ctx context.Context, disconnect, reconnect chan struct{}) {
	ticker := time.NewTicker(5 * time.Second)
	maxWait := 30 * time.Second
	lastConnectionTime := time.Now()
	isDisconnected := false

	for {
		select {
		case <-ticker.C:
			if s.IsConnected() {
				lastConnectionTime = time.Now()
				if isDisconnected {
					if reconnect != nil {
						reconnect <- struct{}{}
					}

					isDisconnected = true
				}

			} else {
				currentWait := time.Now().Sub(lastConnectionTime)
				if currentWait > maxWait {
					isDisconnected = true
					if disconnect != nil {
						log.Infof("not connected to syncthing for %s seconds, sending disconnect notification", currentWait)
						disconnect <- struct{}{}
						lastConnectionTime = time.Now()
					}
				}
			}

		case <-ctx.Done():
			return
		}
	}
}
