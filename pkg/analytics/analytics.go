// Copyright 2021 The Okteto Authors
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
	"regexp"
	"runtime"
	"strings"
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

	upEvent                  = "Up"
	upErrorEvent             = "Up Error"
	durationActivateUpEvent  = "Up Duration Time"
	reconnectEvent           = "Reconnect"
	durationInitialSyncEvent = "Initial Sync Duration Time"
	syncErrorEvent           = "Sync Error"
	syncResetDatabase        = "Sync Reset Database"
	downEvent                = "Down"
	downVolumesEvent         = "DownVolumes"
	pushEvent                = "Push"
	statusEvent              = "Status"
	doctorEvent              = "Doctor"
	buildEvent               = "Build"
	buildTransientErrorEvent = "BuildTransientError"
	deployStackEvent         = "Deploy Stack"
	destroyStackEvent        = "Destroy Stack"
	loginEvent               = "Login"
	initEvent                = "Create Manifest"
	namespaceEvent           = "Namespace"
	namespaceCreateEvent     = "CreateNamespace"
	namespaceDeleteEvent     = "DeleteNamespace"
	previewDeployEvent       = "DeployPreview"
	previewDestroyEvent      = "DestroyPreview"
	execEvent                = "Exec"
	signupEvent              = "Signup"
	disableEvent             = "Disable Analytics"
	stackNotSupportedField   = "Stack Field Not Supported"
)

var (
	mixpanelClient mixpanel.Mixpanel
	clusterType    string
	clusterContext string
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

// SetClusterType sets the cluster type for analytics
func SetClusterType(value string) {
	clusterType = value
}

// SetClusterContext sets the cluster context for analytics
func SetClusterContext(value string) {
	clusterContext = value
}

// TrackInit sends a tracking event to mixpanel when the user creates a manifest
func TrackInit(success bool, language string) {
	props := map[string]interface{}{
		"language": language,
	}
	track(initEvent, success, props)
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

// TrackPreviewDeploy sends a tracking event to mixpanel when the creates a preview environment
func TrackPreviewDeploy(success bool) {
	track(previewDeployEvent, success, nil)
}

// TrackPreviewDestroy sends a tracking event to mixpanel when the deletes a preview environment
func TrackPreviewDestroy(success bool) {
	track(previewDestroyEvent, success, nil)
}

// TrackReconnect sends a tracking event to mixpanel when the development container reconnect
func TrackReconnect(success, swap bool) {
	props := map[string]interface{}{
		"swap": swap,
	}
	track(reconnectEvent, success, props)
}

// TrackSyncError sends a tracking event to mixpanel when the init sync fails
func TrackSyncError() {
	track(syncErrorEvent, false, nil)
}

// TrackSyncError sends a tracking event to mixpanel when the init sync fails
func TrackDurationInitialSync(durationInitialSync time.Duration) {
	props := map[string]interface{}{
		"duration": durationInitialSync,
	}
	track(durationInitialSyncEvent, true, props)
}

// TrackResetDatabase sends a tracking event to mixpanel when the syncthing database is reset
func TrackResetDatabase(success bool) {
	track(syncResetDatabase, success, nil)
}

// TrackUp sends a tracking event to mixpanel when the user activates a development container
func TrackUp(success bool, devName string, interactive, single, swap, divert bool) {
	props := map[string]interface{}{
		"name":          devName,
		"interactive":   interactive,
		"singleService": single,
		"swap":          swap,
		"divert":        divert,
	}
	track(upEvent, success, props)
}

// TrackUpError sends a tracking event to mixpanel when the okteto up command fails
func TrackUpError(success, swap bool) {
	props := map[string]interface{}{
		"swap": swap,
	}
	track(upErrorEvent, success, props)
}

// TrackDurationActivateUp sends a tracking event to mixpanel of the time that has elapsed in the execution of up
func TrackDurationActivateUp(durationActivateUp time.Duration) {
	props := map[string]interface{}{
		"duration": durationActivateUp,
	}
	track(durationActivateUpEvent, true, props)
}

// TrackExec sends a tracking event to mixpanel when the user runs the exec command
func TrackExec(success bool) {
	track(execEvent, success, nil)
}

// TrackDown sends a tracking event to mixpanel when the user deactivates a development container
func TrackDown(success bool) {
	track(downEvent, success, nil)
}

// TrackDownVolumes sends a tracking event to mixpanel when the user deactivates a development container and its volumes
func TrackDownVolumes(success bool) {
	track(downVolumesEvent, success, nil)
}

// TrackPush sends a tracking event to mixpanel when the user pushes a development container
func TrackPush(success bool, oktetoRegistryURL string) {
	props := map[string]interface{}{
		"oktetoRegistryURL": oktetoRegistryURL,
	}
	track(pushEvent, success, props)
}

// TrackStatus sends a tracking event to mixpanel when the user uses the status command
func TrackStatus(success, showInfo bool) {
	props := map[string]interface{}{
		"showInfo": showInfo,
	}
	track(statusEvent, success, props)
}

// TrackDoctor sends a tracking event to mixpanel when the user uses the doctor command
func TrackDoctor(success bool) {
	track(doctorEvent, success, nil)
}

func trackDisable(success bool) {
	track(disableEvent, success, nil)
}

// TrackBuild sends a tracking event to mixpanel when the user builds on remote
func TrackBuild(oktetoBuilkitURL string, success bool) {
	props := map[string]interface{}{
		"oktetoBuilkitURL": oktetoBuilkitURL,
	}
	track(buildEvent, success, props)
}

// TrackBuildTransientError sends a tracking event to mixpanel when the user build fails because of a transient error
func TrackBuildTransientError(oktetoBuilkitURL string, success bool) {
	props := map[string]interface{}{
		"oktetoBuilkitURL": oktetoBuilkitURL,
	}
	track(buildTransientErrorEvent, success, props)
}

// TrackDeployStack sends a tracking event to mixpanel when the user deploys a stack
func TrackDeployStack(success, isCompose bool) {
	props := map[string]interface{}{
		"isCompose": isCompose,
	}
	track(deployStackEvent, success, props)
}

// TrackDestroyStack sends a tracking event to mixpanel when the user destroys a stack
func TrackDestroyStack(success bool) {
	track(destroyStackEvent, success, nil)
}

// TrackLogin sends a tracking event to mixpanel when the user logs in
func TrackLogin(success bool, name, email, oktetoID, externalID string) {
	if !isEnabled() {
		return
	}

	track(loginEvent, success, nil)
	if name == "" {
		name = externalID
	}

	if err := mixpanelClient.Update(oktetoID, &mixpanel.Update{
		Operation: "$set",
		Properties: map[string]interface{}{
			"$name":    name,
			"$email":   email,
			"oktetoId": oktetoID,
			"githubId": externalID,
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

func TrackStackWarnings(warnings []string) {
	re := regexp.MustCompile(`\[(.*?)\]`)
	for _, warning := range warnings {
		found := re.FindString(warning)
		if found != "" {
			warning = strings.Replace(warning, found, "", 1)
		}
		props := map[string]interface{}{
			"field": warning,
		}
		track(stackNotSupportedField, true, props)
	}
}

func track(event string, success bool, props map[string]interface{}) {
	if !isEnabled() {
		return
	}
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
	if clusterType != "" {
		props["clusterType"] = clusterType
	}
	if clusterContext != "" {
		props["clusterContext"] = clusterContext
	}

	e := &mixpanel.Event{Properties: props}
	trackID := getTrackID()
	if err := mixpanelClient.Track(trackID, event, e); err != nil {
		log.Infof("Failed to send analytics: %s", err)
	}
}

func getFlagPath() string {
	return filepath.Join(config.GetOktetoHome(), ".noanalytics")
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
		log.Info("failed to save the machine id")
		mid = "na"
	}

	return mid
}

func generateMachineID() string {
	mid, err := machineid.ProtectedID("okteto")
	if err != nil {
		log.Infof("failed to generate a machine id: %s", err)
		mid = "na"
	}

	return mid
}
