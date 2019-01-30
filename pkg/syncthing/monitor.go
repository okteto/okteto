package syncthing

import (
	"context"
	"time"
)

var consecutiveErrors = 0

const maxConsecutiveErrors = 3

//Monitor verifies that syncthing is not in a disconnected state. If so, it sends a message to the
// disconnected channel and exits
func (s *Syncthing) Monitor(ctx context.Context, disconnected chan struct{}) {
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-ticker.C:
			if !s.isConnectedToRemote() {
				consecutiveErrors++
				if consecutiveErrors > maxConsecutiveErrors {
					disconnected <- struct{}{}
					return
				}
			} else {
				consecutiveErrors = 0
			}

		case <-ctx.Done():
			return
		}
	}
}
