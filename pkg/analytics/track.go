// Copyright 2023 The Okteto Authors
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

	downEvent                     = "Down"
	downVolumesEvent              = "DownVolumes"
	restartEvent                  = "Restart Services"
	statusEvent                   = "Status"
	logsEvent                     = "Logs"
	doctorEvent                   = "Doctor"
	buildEvent                    = "Build"
	buildWithManifestVsDockerfile = "BuildWithManifestVsDockerfile"
	buildTransientErrorEvent      = "BuildTransientError"
	destroyEvent                  = "Destroy"
	deployStackEvent              = "Deploy Stack"
	namespaceEvent                = "Namespace"
	namespaceCreateEvent          = "CreateNamespace"
	namespaceDeleteEvent          = "DeleteNamespace"
	previewDeployEvent            = "DeployPreview"
	previewDestroyEvent           = "DestroyPreview"
	execEvent                     = "Exec"
	signupEvent                   = "Signup"
	contextEvent                  = "Context"
	disableEvent                  = "Disable Analytics"
	stackNotSupportedField        = "Stack Field Not Supported"
	buildPullErrorEvent           = "BuildPullError"
	deleteContexts                = "Contexts Deletion"
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

	mixpanelClient = mixpanel.NewFromClient(c, mixpanelToken, "https://analytics.okteto.com")
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
func TrackPreviewDeploy(success bool, scope string) {
	props := map[string]interface{}{
		"scope": scope,
	}
	track(previewDeployEvent, success, props)
}

// TrackPreviewDestroy sends a tracking event to mixpanel when the deletes a preview environment
func TrackPreviewDestroy(success bool) {
	track(previewDestroyEvent, success, nil)
}

// TrackExecMetadata is the metadata added to execEvent
type TrackExecMetadata struct {
	Mode                   string
	FirstArgIsDev          bool
	Success                bool
	IsOktetoRepository     bool
	IsInteractive          bool
	HasBuildSection        bool
	HasDeploySection       bool
	HasDependenciesSection bool
}

// TrackExec sends a tracking event to mixpanel when the user runs the exec command
func TrackExec(m *TrackExecMetadata) {
	props := map[string]interface{}{
		"isFirstArgDev": m.FirstArgIsDev,
		// defined dict for Exec event
		"mode":                   m.Mode,
		"isOktetoRepository":     m.IsOktetoRepository,
		"isInteractive":          m.IsInteractive,
		"hasDependenciesSection": m.HasDependenciesSection,
		"hasBuildSection":        m.HasBuildSection,
		"hasDeploySection":       m.HasDeploySection,
	}
	track(execEvent, m.Success, props)
}

// TrackRestart sends a tracking event to mixpanel when the user restarts a development environment
func TrackRestart(success bool) {
	track(restartEvent, success, nil)
}

// TrackLogs sends a tracking event to mixpanel when the command okteto logs is executed
func TrackLogs(success, all bool) {
	props := map[string]interface{}{
		"all": all,
	}
	track(logsEvent, success, props)
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

func TrackBuildWithManifestVsDockerfile(isDockerfile bool) {
	props := map[string]interface{}{
		"isDockerfile": isDockerfile,
	}
	track(buildWithManifestVsDockerfile, true, props)

}

// TrackBuild sends a tracking event to mixpanel when the user builds on remote
func TrackBuild(success bool) {
	track(buildEvent, success, nil)
}

// TrackBuildTransientError sends a tracking event to mixpanel when the user build fails because of a transient error
func TrackBuildTransientError(success bool) {
	track(buildTransientErrorEvent, success, nil)
}

// TrackDeployStack sends a tracking event to mixpanel when the user deploys a stack
func TrackDeployStack(success, isCompose bool) {
	props := map[string]interface{}{
		"isCompose":  isCompose,
		"deployType": "stack",
	}
	track(deployStackEvent, success, props)
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
func TrackBuildPullError(success bool) {
	track(buildPullErrorEvent, success, nil)
}

// TrackContextDelete sends a tracking event to mixpanel indicating one or more context have been deleted
func TrackContextDelete(ctxs int, success bool) {
	props := map[string]interface{}{
		"totalContextsDeleted": ctxs,
	}
	track(deleteContexts, success, props)
}

func track(event string, success bool, props map[string]interface{}) {
	if !get().Enabled {
		oktetoLog.Info("failed to send analytics: analytics has been disabled")
		return
	}

	if !okteto.IsContextInitialized() {
		oktetoLog.Info("failed to send analytics: okteto context not initialized")
		return
	}

	if disabledByOktetoAdmin() {
		oktetoLog.Info("failed to send analytics: analytics disabled by admin")
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
	props["machine_id"] = get().MachineID
	if okteto.GetContext().ClusterType != "" {
		props["clusterType"] = okteto.GetContext().ClusterType
	}

	props["source"] = origin
	props["origin"] = origin
	props["success"] = success
	props["contextType"] = getContextType()
	props["isOkteto"] = okteto.GetContext().IsOkteto
	if termType := os.Getenv(model.TermEnvVar); termType == "" {
		props["term-type"] = "other"
	} else {
		props["term-type"] = termType
	}

	props["context"] = okteto.GetContext().CompanyName
	props["isTrial"] = okteto.GetContext().IsTrial

	e := &mixpanel.Event{Properties: props}
	if err := mixpanelClient.Track(getTrackID(), event, e); err != nil {
		oktetoLog.Infof("Failed to send analytics: %s", err)
	}
}

func disabledByOktetoAdmin() bool {
	return !okteto.GetContext().Analytics
}
