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
	trackID        string
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
func TrackLogin(name, email, oktetoID, githubID, version string, success bool) {
	if !isEnabled() {
		return
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

// TrackSignup sends a tracking event to mixpanel when the user signs up
func TrackSignup(userID, version string, success bool) {
	if err := mixpanelClient.Alias(getMachineID(), userID); err != nil {
		log.Errorf("failed to alias %s to %s", getMachineID(), userID)
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

		origin, ok := os.LookupEnv("OKTETO_ORIGIN")
		if !ok {
			origin = "cli"
		}

		e := &mixpanel.Event{
			Properties: map[string]interface{}{
				"$os":               mpOS,
				"version":           version,
				"$referring_domain": okteto.GetURL(),
				"image":             image,
				"machine_id":        getMachineID(),
				"origin":            origin,
				"success":           success,
			},
		}

		trackID := getTrackID()
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

func getTrackID() string {
	uid := okteto.GetUserID()
	if len(uid) > 0 {
		return uid
	}

	return getMachineID()
}

func getMachineID() string {
	mid := okteto.GetMachineID()
	if len(mid) > 0 {
		return mid
	}

	mid = generateMachineID()
	if err := okteto.SaveMachineID(mid); err != nil {
		log.Debugf("failed to save the machine id")
		mid = "na"
	}

	return mid
}

func generateMachineID() string {
	mid, err := machineid.ProtectedID("okteto")
	if err != nil {
		log.Debugf("failed to generate a machine id")
		mid = "na"
	}

	return mid
}
