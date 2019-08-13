package analytics

import (
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/denisbrodbeck/machineid"
	"github.com/dukex/mixpanel"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
)

const (
	mixpanelToken = "92fe782cdffa212d8f03861fbf1ea301"

	upEvent              = "Up"
	downEvent            = "Down"
	loginEvent           = "Login"
	initEvent            = "Create Manifest"
	namespaceEvent       = "Namespace"
	namespaceCreateEvent = "CreateNamespace"
	namespaceDeleteEvent = "DeleteNamespace"
	execEvent            = "Exec"
	signupEvent          = "Signup"
	disableEvent         = "Disable Analytics"
)

var (
	mixpanelClient mixpanel.Mixpanel
	machineID      string
)

func init() {
	c := &http.Client{
		Timeout: time.Second * 5,
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout: 5 * time.Second,
			}).Dial,
			TLSHandshakeTimeout: 5 * time.Second,
		},
	}

	mixpanelClient = mixpanel.NewFromClient(c, mixpanelToken, "")

	var err error
	machineID, err = machineid.ProtectedID("okteto")
	if err != nil {
		log.Debugf("failed to generate a machine id")
		machineID = "na"
	}
}

// TrackInit sends a tracking event to mixpanel when the user creates a manifest
func TrackInit(language, image, version string, success bool) {
	track(initEvent, version, image, success)
}

// TrackNamespace sends a tracking event to mixpanel when the user changes a namespace
func TrackNamespace(version string, success bool) {
	track(namespaceEvent, version, "", success)
}

// TrackCreateNamespace sends a tracking event to mixpanel when the creates a namespace
func TrackCreateNamespace(version string, success bool) {
	track(namespaceCreateEvent, version, "", success)
}

// TrackDeleteNamespace sends a tracking event to mixpanel when the user deletes a namespace
func TrackDeleteNamespace(version string, success bool) {
	track(namespaceDeleteEvent, version, "", success)
}

// TrackUp sends a tracking event to mixpanel when the user activates a development environment
func TrackUp(image, version string, success bool) {
	track(upEvent, version, image, success)
}

// TrackExec sends a tracking event to mixpanel when the user runs the exec command
func TrackExec(image, version string, success bool) {
	track(execEvent, version, image, success)
}

// TrackDown sends a tracking event to mixpanel when the user deactivates a development environment
func TrackDown(version string, success bool) {
	track(downEvent, version, "", success)
}

func trackDisable(version string, success bool) {
	track(disableEvent, version, "", success)
}

// TrackLogin sends a tracking event to mixpanel when the user logs in
func TrackLogin(name, email, oktetoID, githubID, version string, isNew bool, success bool) {
	if !isEnabled() {
		return
	}

	if isNew {
		trackSignup(version, success)
	}

	track(loginEvent, version, "", success)
	if len(name) == 0 {
		name = githubID
	}

	if err := mixpanelClient.Update(oktetoID, &mixpanel.Update{
		Operation: "$set",
		Properties: map[string]interface{}{
			"$name":    name,
			"$email":   email,
			"oktetoId": oktetoID,
			"githubId": githubID,
		},
	}); err != nil {
		log.Infof("failed to update user: %s", err)
	}
}

func trackSignup(version string, success bool) {
	trackID := okteto.GetUserID()
	if len(trackID) == 0 {
		log.Errorf("userID wasn't set for a new user")
	} else {
		if err := mixpanelClient.Alias(machineID, trackID); err != nil {
			log.Errorf("failed to alias %s to %s", machineID, trackID)
		}
	}

	track(signupEvent, version, "", success)
}

func track(event, version, image string, success bool) {
	if isEnabled() {
		mpOS := ""
		switch runtime.GOOS {
		case "darwin":
			mpOS = "Mac OS X"
		case "windows":
			mpOS = "Windows"
		case "linux":
			mpOS = "Linux"
		}

		e := &mixpanel.Event{
			Properties: map[string]interface{}{
				"$os":               mpOS,
				"version":           version,
				"$referring_domain": okteto.GetURL(),
				"image":             image,
				"machine_id":        machineID,
				"origin":            "cli",
				"success":           success,
			},
		}

		trackID := okteto.GetUserID()
		if len(trackID) == 0 {
			trackID = machineID
		}

		if err := mixpanelClient.Track(trackID, event, e); err != nil {
			log.Infof("Failed to send analytics: %s", err)
		}
	} else {
		log.Debugf("not sending event for %s", event)
	}
}

func getFlagPath() string {
	return filepath.Join(config.GetHome(), ".noanalytics")
}

// Disable disables analytics
func Disable(version string) error {
	var _, err = os.Stat(getFlagPath())
	if os.IsNotExist(err) {
		var file, err = os.Create(getFlagPath())
		if err != nil {
			trackDisable(version, false)
			return err
		}

		defer file.Close()
	}

	trackDisable(version, true)
	return nil
}

// Enable enables analytics
func Enable(version string) error {
	var _, err = os.Stat(getFlagPath())
	if os.IsNotExist(err) {
		return nil
	}

	return os.Remove(getFlagPath())
}

func isEnabled() bool {
	if _, err := os.Stat(getFlagPath()); !os.IsNotExist(err) {
		return false
	}

	return true
}
