package syncthing

import (
	"context"
	"sync"
	"time"

	"github.com/okteto/okteto/pkg/log"
)

// isConnected returns true if it can ping the remote syncthing
func (s *Syncthing) isConnected() bool {
	_, err := s.APICall("rest/system/ping", "GET", 200, nil, false, nil)
	if err != nil {
		log.Infof("syncthing ping failed: %s", err)
		return false
	}

	return true
}

// Monitor will send a message to disconnected if remote syncthing is disconnected for more than 10 seconds.
func (s *Syncthing) Monitor(ctx context.Context, wg *sync.WaitGroup, disconnect chan struct{}) {
	wg.Add(1)
	defer wg.Done()
	ticker := time.NewTicker(3 * time.Second)
	connected := true
	for {
		select {
		case <-ticker.C:
			if s.isConnected() {
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
