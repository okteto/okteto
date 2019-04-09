package syncthing

import (
	"context"
	"sync"
	"time"
)

// IsConnected returns true if it can ping the remote syncthing
func (s *Syncthing) IsConnected() bool {
	_, err := s.APICall("rest/system/ping", "GET", 200, nil, false)
	if err != nil {
		return false
	}
	return true
}

// Monitor will send a message to disconnected if remote syncthing is disconnected for more than 10 seconds.
func (s *Syncthing) Monitor(ctx context.Context, wg *sync.WaitGroup, disconnect chan struct{}) {
	wg.Add(1)
	defer wg.Done()
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-ticker.C:
			if !s.IsConnected() {
				if !s.IsConnected() {
					disconnect <- struct{}{}
					return
				}
			}
		case <-ctx.Done():
			return
		}
	}
}
