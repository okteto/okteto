package syncthing

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"cli/cnd/pkg/log"
)

// IsConnected returns true if it can ping the remote syncthing
func (s *Syncthing) IsConnected() bool {
	if !s.Primary {
		return true
	}
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

// GetFolderSyncCompletion returns the percentage of sync completion
func (s *Syncthing) GetFolderSyncCompletion(deployment, container string) (float64, error) {
	folderName := fmt.Sprintf("cnd-%s-%s", deployment, container)
	c, err := s.getDBCompletion(folderName)
	if err != nil {
		log.Debugf("failed to call the db/completion API: %s", err)
	} else {
		if c >= 100 {
			return c, nil
		}
	}

	p := make(map[string]string)
	p["events"] = "FolderCompletion"
	p["device"] = s.RemoteDeviceID
	p["folder"] = folderName
	p["timeout"] = "0"

	body, err := s.APICall("rest/events", "GET", 200, p, true)
	if err != nil {
		return 0, fmt.Errorf("error getting syncthing state: %s", err)
	}

	var events []event
	if err := json.Unmarshal(body, &events); err != nil {
		return 0, fmt.Errorf("error unmarshalling syncthing state: %s", err)
	}

	for i := len(events) - 1; i >= 0; i-- {
		e := events[i]
		if e.Type != "FolderCompletion" || e.Data.Folder != folderName {
			log.Infof("got a bad event: %+v", e)
			continue
		}

		log.Debugf("got completion for %s - %2.f, global %d", e.Data.Folder, e.Data.Completion, e.Data.GlobalBytes)
		if e.Data.GlobalBytes != 0 {
			return e.Data.Completion, nil
		}
	}

	return 100, nil
}

func (s *Syncthing) getDBCompletion(folderName string) (float64, error) {
	p := make(map[string]string)
	p["events"] = "FolderCompletion"
	p["device"] = s.RemoteDeviceID
	p["folder"] = folderName

	body, err := s.APICall("rest/db/completion", "GET", 200, p, true)
	if err != nil {
		return 0, fmt.Errorf("error getting syncthing state: %s", err)
	}

	var c completion
	if err := json.Unmarshal(body, &c); err != nil {
		return 0, fmt.Errorf("error unmarshalling syncthing state: %s", err)
	}

	return c.Completion, nil
}

// GetErrors returns the sync errors
func (s *Syncthing) GetErrors() ([]string, error) {
	body, err := s.APICall("rest/system/error", "GET", 200, nil, true)
	if err != nil {
		return nil, fmt.Errorf("error getting syncthing errors: %s", err)
	}

	var errors syncthingErrors
	if err := json.Unmarshal(body, &errors); err != nil {
		return nil, fmt.Errorf("error getting syncthing errors: %s", err)
	}

	parsedErrors := make([]string, len(errors.Errors))
	for i := range parsedErrors {
		sp := strings.Split(errors.Errors[i].Message, ":")
		if len(sp) == 0 {
			parsedErrors[i] = sp[0]
			log.Infof("error with unexpected format: %s", errors.Errors)
		} else {
			parsedErrors[i] = strings.TrimSpace(sp[1])
		}

	}

	return parsedErrors, nil
}
