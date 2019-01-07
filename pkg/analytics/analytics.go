package analytics

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/denisbrodbeck/machineid"
	"github.com/okteto/cnd/pkg/model"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
)

//EventName event name
type EventName string

type event struct {
	//ActionID to correlate different events
	ActionID string `json:"action"`

	//Event name of the event
	Event EventName `json:"event"`

	//User local id of the client
	User string `json:"uid"`

	//Time time of the event
	Time int64 `json:"time"`

	//Version of the cli
	Version string `json:"version"`
}

const endpoint = "https://us-central1-okteto-prod.cloudfunctions.net/cnd-analytics"

var (
	client   http.Client
	userID   string
	flagPath string
	wg       = sync.WaitGroup{}
)

const (
	// EventUp event for up
	EventUp = "up"

	// EventUpEnd event for when up finishes
	EventUpEnd = "upend"

	// EventExec event for exec
	EventExec = "exec"

	// EventExecEnd event for when exec finishes
	EventExecEnd = "execupend"

	// EventRun event for run
	EventRun = "run"

	// EventRunEnd event for when run finishes
	EventRunEnd = "runenc"
)

func init() {
	client = http.Client{
		Timeout: 65 * time.Second,
	}

	var err error
	userID, err = machineid.ProtectedID("cnd")
	if err != nil {
		log.Debugf("failed to generate a machine id")
		userID = "na"
	}
}

// NewActionID returns an action
func NewActionID() string {
	return uuid.NewV4().String()
}

// Send send analytics event
func Send(e EventName, actionID string) {
	go func() {
		wg.Add(1)
		defer wg.Done()

		ev := event{
			ActionID: actionID,
			Event:    e,
			Time:     time.Now().UTC().Unix(),
			Version:  model.VersionString,
			User:     userID,
		}

		data, err := json.Marshal(ev)
		if err != nil {
			log.Debugf("failed to marshall analytic event: %s", err)
			return
		}

		if !isEnabled() {
			return
		}

		log.Debugf("sending: %s", string(data))
		req, _ := http.NewRequest("POST", endpoint, bytes.NewBuffer(data))
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			log.Debugf("failed to send the analytics: %s", err)
			return
		}

		if resp.StatusCode > 300 {
			log.Debugf("analytics fail to process request: %d", resp.StatusCode)
			return
		}

		log.Debugf("analytics sucess: %d", resp.StatusCode)
	}()
}

// Disable disables analytics
func Disable() error {
	var _, err = os.Stat(flagPath)
	if os.IsNotExist(err) {
		var file, err = os.Create(flagPath)
		if err != nil {
			return err
		}

		defer file.Close()
	}

	return nil
}

// Enable enables analytics
func Enable() error {
	var _, err = os.Stat(flagPath)
	if os.IsNotExist(err) {
		return nil
	}

	return os.Remove(flagPath)
}

func isEnabled() bool {
	if _, err := os.Stat(flagPath); !os.IsNotExist(err) {
		return false
	}

	return true
}

func Wait() {
	wg.Wait()
}
