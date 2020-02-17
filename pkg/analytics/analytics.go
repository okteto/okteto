// Copyright 2020 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	// skipcq GSC-G101
	// This is mixpanel's public token, is needed to send analytics to the project
	mixpanelToken = "92fe782cdffa212d8f03861fbf1ea301"

	upEvent              = "Up"
	upErrorEvent         = "Up Error"
	reconnectEvent       = "Reconnect"
	syncErrorEvent       = "Sync Error"
	downEvent            = "Down"
	downVolumesEvent     = "DownVolumes"
	redeployEvent        = "Redeploy"
	buildEvent           = "Build"
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
func TrackInit(success bool) {
	track(initEvent, success, nil)
}

// TrackNamespace sends a tracking event to mixpanel when the user changes a namespace
func TrackNamespace(success bool) {
	track(namespaceEvent, success, nil)
}

// TrackCreateNamespace sends a tracking event to mixpanel when the creates a namespace
func TrackCreateNamespace(success bool) {
	track(namespaceCreateEvent, success, nil)
}

// TrackDeleteNamespace sends a tracking event to mixpanel when the user deletes a namespace
func TrackDeleteNamespace(success bool) {
	track(namespaceDeleteEvent, success, nil)
}

// TrackReconnect sends a tracking event to mixpanel when the dev environment reconnect
func TrackReconnect(success bool, clusterType string, swap bool) {
	props := map[string]interface{}{
		"clusterType": clusterType,
		"swap":        swap,
	}
	track(reconnectEvent, success, props)
}

// TrackSyncError sends a tracking event to mixpanel when the init sync fails
func TrackSyncError() {
	track(syncErrorEvent, false, nil)
}

// TrackUp sends a tracking event to mixpanel when the user activates a development environment
func TrackUp(success bool, dev, clusterType string, single, swap, remote bool) {
	props := map[string]interface{}{
		"devEnvironmentName": dev,
		"clusterType":        clusterType,
		"singleService":      single,
		"swap":               swap,
		"remote":             remote,
	}
	track(upEvent, success, props)
}

// TrackUpError sends a tracking event to mixpanel when the okteto up command fails
func TrackUpError(success bool, swap bool) {
	props := map[string]interface{}{
		"swap": swap,
	}
	track(upErrorEvent, success, props)
}

// TrackExec sends a tracking event to mixpanel when the user runs the exec command
func TrackExec(success bool) {
	track(execEvent, success, nil)
}

// TrackDown sends a tracking event to mixpanel when the user deactivates a development environment
func TrackDown(success bool) {
	track(downEvent, success, nil)
}

// TrackDownVolumes sends a tracking event to mixpanel when the user deactivates a development environment and its volumes
func TrackDownVolumes(success bool) {
	track(downVolumesEvent, success, nil)
}

// TrackRedeploy sends a tracking event to mixpanel when the user redeploys a development environment
func TrackRedeploy(success, isOktetoNamespace bool) {
	props := map[string]interface{}{
		"isOktetoNamespace": isOktetoNamespace,
	}
	track(redeployEvent, success, props)
}

func trackDisable(success bool) {
	track(disableEvent, success, nil)
}

// TrackBuild sends a tracking event to mixpanel when the user builds on remote
func TrackBuild(success bool) {
	track(buildEvent, success, nil)
}

// TrackLogin sends a tracking event to mixpanel when the user logs in
func TrackLogin(success bool, name, email, oktetoID, githubID string) {
	if !isEnabled() {
		return
	}

	track(loginEvent, success, nil)
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
func TrackSignup(success bool, userID string) {
	if err := mixpanelClient.Alias(getMachineID(), userID); err != nil {
		log.Errorf("failed to alias %s to %s", getMachineID(), userID)
	}

	track(signupEvent, success, nil)
}

func track(event string, success bool, props map[string]interface{}) {
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

		if props == nil {
			props = map[string]interface{}{}
		}
		props["$os"] = mpOS
		props["version"] = config.VersionString
		props["$referring_domain"] = okteto.GetURL()
		props["machine_id"] = getMachineID()
		props["origin"] = origin
		props["success"] = success

		e := &mixpanel.Event{Properties: props}
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
	trackDisable(true)
	if os.IsNotExist(err) {
		var file, err = os.Create(getFlagPath())
		if err != nil {
			trackDisable(false)
			return err
		}

		defer file.Close()
	}
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
