package syncthing

import (
	"context"
	"time"

	"github.com/okteto/okteto/pkg/log"
)

// isConnected returns true if it can ping the remote syncthing
func (s *Syncthing) isConnected(ctx context.Context) bool {
	_, err := s.APICall(ctx, "rest/system/ping", "GET", 200, nil, false, nil)
	if err != nil {
		log.Infof("syncthing ping failed: %s", err)
		return false
	}

	return true
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
