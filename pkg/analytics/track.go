// Copyright 2022 The Okteto Authors
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
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/dukex/mixpanel"
	"github.com/okteto/okteto/pkg/config"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
)

const (
	// skipcq GSC-G101
	// This is mixpanel's public token, is needed to send analytics to the project
	mixpanelToken = "92fe782cdffa212d8f03861fbf1ea301"

	upEvent                  = "Up"
	upErrorEvent             = "Up Error"
	manifestHasChangedEvent  = "Manifest Has Changed"
	durationActivateUpEvent  = "Up Duration Time"
	reconnectEvent           = "Reconnect"
	durationInitialSyncEvent = "Initial Sync Duration Time"
	syncErrorEvent           = "Sync Error"
	syncResetDatabase        = "Sync Reset Database"
	downEvent                = "Down"
	downVolumesEvent         = "DownVolumes"
	pushEvent                = "Push"
	restartEvent             = "Restart Services"
	statusEvent              = "Status"
	doctorEvent              = "Doctor"
	buildEvent               = "Build"
	buildTransientErrorEvent = "BuildTransientError"
	deployEvent              = "Deploy"
	destroyEvent             = "Destroy"
	deployStackEvent         = "Deploy Stack"
	destroyStackEvent        = "Destroy Stack"
	loginEvent               = "Login"
	initEvent                = "Create Manifest"
	kubeconfigEvent          = "Kubeconfig"
	namespaceEvent           = "Namespace"
	namespaceCreateEvent     = "CreateNamespace"
	namespaceDeleteEvent     = "DeleteNamespace"
	previewDeployEvent       = "DeployPreview"
	previewDestroyEvent      = "DestroyPreview"
	execEvent                = "Exec"
	signupEvent              = "Signup"
	contextEvent             = "Context"
	contextUseNamespaceEvent = "Context Use-namespace"
	disableEvent             = "Disable Analytics"
	stackNotSupportedField   = "Stack Field Not Supported"
	buildPullErrorEvent      = "BuildPullError"
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
func TrackInit(success bool, language string) {
	props := map[string]interface{}{
		"language": language,
	}
	track(initEvent, success, props)
}

// TrackKubeconfig sends a tracking event to mixpanel when the user use the kubeconfig command
func TrackKubeconfig(success bool) {
	track(kubeconfigEvent, success, nil)
}

// TrackNamespace sends a tracking event to mixpanel when the user changes a namespace
func TrackNamespace(success, withArg bool) {
	props := map[string]interface{}{
		"withArg": withArg,
	}
	track(namespaceEvent, success, props)
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
func TrackReconnect(success bool) {
	track(reconnectEvent, success, nil)
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

type TrackUpMetadata struct {
	IsInteractive          bool
	IsOktetoRepository     bool
	HasDependenciesSection bool
	HasBuildSection        bool
	HasDeploySection       bool
	Success                bool
}

// TrackUp sends a tracking event to mixpanel when the user activates a development container
func TrackUp(m TrackUpMetadata) {
	props := map[string]interface{}{
		"isInteractive":          m.IsInteractive,
		"isOktetoRepository":     m.IsOktetoRepository,
		"hasDependenciesSection": m.HasDependenciesSection,
		"hasBuildSection":        m.HasBuildSection,
		"hasDeploySection":       m.HasDeploySection,
	}
	track(upEvent, m.Success, props)
}

// TrackUpError sends a tracking event to mixpanel when the okteto up command fails
func TrackUpError(success bool) {
	track(upErrorEvent, success, nil)
}

// TrackManifestHasChanged sends a tracking event to mixpanel when the okteto up command fails because manifest has changed
func TrackManifestHasChanged(success bool) {
	track(manifestHasChangedEvent, success, nil)
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

// TrackRestart sends a tracking event to mixpanel when the user restarts a development environment
func TrackRestart(success bool) {
	track(restartEvent, success, nil)
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
func TrackDeployStack(success, isCompose, isOktetoRepo bool) {
	props := map[string]interface{}{
		"isCompose":          isCompose,
		"deployType":         "stack",
		"isOktetoRepository": isOktetoRepo,
	}
	track(deployStackEvent, success, props)
}

// TrackDestroyStack sends a tracking event to mixpanel when the user destroys a stack
func TrackDestroyStack(success bool) {
	track(destroyStackEvent, success, nil)
}

type TrackDeployMetadata struct {
	Success                bool
	IsOktetoRepo           bool
	Err                    error
	Duration               time.Duration
	PipelineType           model.Archetype
	DeployType             string
	IsPreview              bool
	HasDependenciesSection bool
	HasBuildSection        bool
}

// TrackDeploy sends a tracking event to mixpanel when the user deploys a pipeline
func TrackDeploy(m TrackDeployMetadata) {
	if m.PipelineType == "" {
		m.PipelineType = "pipeline"
	}
	props := map[string]interface{}{
		"pipelineType":           m.PipelineType,
		"isOktetoRepository":     m.IsOktetoRepo,
		"duration":               m.Duration.Seconds(),
		"deployType":             m.DeployType,
		"isPreview":              m.IsPreview,
		"hasDependenciesSection": m.HasDependenciesSection,
		"hasBuildSection":        m.HasBuildSection,
	}
	if m.Err != nil {
		props["error"] = m.Err.Error()
	}
	track(deployEvent, m.Success, props)
}

// TrackDestroy sends a tracking event to mixpanel when the user destroys a pipeline from local
func TrackDestroy(success bool) {
	track(destroyEvent, success, nil)
}

// TrackLogin sends a tracking event to mixpanel when the user logs in
func TrackLogin(success bool) {
	track(loginEvent, success, nil)
}

// TrackSignup sends a tracking event to mixpanel when the user signs up
func TrackSignup(success bool, userID string) {
	if err := mixpanelClient.Alias(get().MachineID, userID); err != nil {
		oktetoLog.Errorf("failed to alias %s to %s", get().MachineID, userID)
	}

	track(signupEvent, success, nil)
}

// TrackContext sends a tracking event to mixpanel when the user use context in
func TrackContext(success bool) {
	if config.RunningInInstaller() {
		return
	}
	track(contextEvent, success, nil)
}

// TrackContextUseNamespace sends a tracking event to mixpanel when the user use context in
func TrackContextUseNamespace(success bool) {
	track(contextUseNamespaceEvent, success, nil)
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

// TrackBuildPullError sends a tracking event to mixpanel when the build was success but the image can't be pulled from registry
func TrackBuildPullError(oktetoBuilkitURL string, success bool) {
	props := map[string]interface{}{
		"oktetoBuilkitURL": oktetoBuilkitURL,
	}
	track(buildPullErrorEvent, success, props)
}

func track(event string, success bool, props map[string]interface{}) {
	if !get().Enabled {
		return
	}

	if !okteto.IsContextInitialized() || (!okteto.Context().Analytics && !okteto.IsOktetoCloud()) {
		return
	}
	// skip events from nested okteto deploys and manifest dependencies
	origin := config.GetDeployOrigin()
	if origin == "okteto-deploy" {
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

	if props == nil {
		props = map[string]interface{}{}
	}
	props["$os"] = mpOS
	props["version"] = config.VersionString
	props["$referring_domain"] = okteto.Context().Name
	props["machine_id"] = get().MachineID
	if okteto.Context().ClusterType != "" {
		props["clusterType"] = okteto.Context().ClusterType
	}

	props["source"] = origin
	props["origin"] = origin
	props["success"] = success
	props["contextType"] = getContextType(okteto.Context().Name)
	props["context"] = okteto.Context().Name
	props["cluster"] = okteto.Context().Name
	props["isOkteto"] = okteto.Context().IsOkteto
	if termType := os.Getenv(model.TermEnvVar); termType == "" {
		props["term-type"] = "other"
	} else {
		props["term-type"] = termType
	}

	e := &mixpanel.Event{Properties: props}
	if err := mixpanelClient.Track(getTrackID(), event, e); err != nil {
		oktetoLog.Infof("Failed to send analytics: %s", err)
	}
}
