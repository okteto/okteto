package syncthing

import (
	"encoding/json"

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
