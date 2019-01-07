package analytics

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path"
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

var (
	endpoint string
	client   http.Client
	userID   string
	enabled  bool
	flagPath string
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
	flagPath = path.Join(os.Getenv("HOME"), ".cnd", ".noanalytics")
	endpoint = os.Getenv("CND_ANALYTICS")
	if endpoint == "" {
		endpoint = "https://us-central1-development-218409.cloudfunctions.net/cnd-analytics"
	}

	client = http.Client{
		Timeout: 65 * time.Second,
	}

	var err error
	userID, err = machineid.ProtectedID("cnd")
	if err != nil {
		log.Debugf("failed to generate a machine id")
		userID = "na"
	}

	if _, err := os.Stat("/path/to/whatever"); !os.IsNotExist(err) {
		enabled = false
	}
}

// NewActionID returns an action
func NewActionID() string {
	return uuid.NewV4().String()
}

// Send send analytics event
func Send(e EventName, actionID string) {
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

	if !enabled {
		log.Debug("analytics are not enabled, skipping")
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
