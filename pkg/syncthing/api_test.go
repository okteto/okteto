package syncthing

import (
	"encoding/json"
	"testing"
)

func TestMarshallResponse(t *testing.T) {
	body := []byte(`{
		"connections": {
		  "ABKAVQF-RUO4CYO-FSC2VIP-VRX4QDA-TQQRN2J-MRDXJUC-FXNWP6N-S6ZSAAR": {
			"address": "",
			"at": "0001-01-01T00:00:00Z",
			"clientVersion": "",
			"connected": false,
			"inBytesTotal": 0,
			"outBytesTotal": 0,
			"paused": false,
			"type": ""
		  },
		  "ATOPHFJ-VPVLDFY-QVZDCF2-OQQ7IOW-OG4DIXF-OA7RWU3-ZYA4S22-SI4XVAU": {
			"address": "[::1]:51761",
			"at": "2019-01-27T02:45:20.406724-08:00",
			"clientVersion": "v1.0.0",
			"connected": true,
			"inBytesTotal": 161,
			"outBytesTotal": 181,
			"paused": false,
			"type": "tcp-client"
		  }
		},
		"total": {
		  "address": "",
		  "at": "2019-01-27T02:45:20.406734-08:00",
		  "clientVersion": "",
		  "connected": false,
		  "inBytesTotal": 161,
		  "outBytesTotal": 181,
		  "paused": false,
		  "type": ""
		}
	  }`)

	var conns syncthingConnections
	if err := json.Unmarshal(body, &conns); err != nil {
		t.Fatalf("%s\n%s", body, err)
	}

	if len(conns.Connections) == 0 {
		t.Fatalf("didn't parse the connections")
	}

	if _, ok := conns.Connections["ATOPHFJ-VPVLDFY-QVZDCF2-OQQ7IOW-OG4DIXF-OA7RWU3-ZYA4S22-SI4XVAU"]; !ok {
		t.Error("missing entry")
	}

	if !conns.Connections["ATOPHFJ-VPVLDFY-QVZDCF2-OQQ7IOW-OG4DIXF-OA7RWU3-ZYA4S22-SI4XVAU"].Connected {
		t.Error("missing entry value")
	}
}
