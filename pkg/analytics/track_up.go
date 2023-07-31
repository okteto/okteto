package analytics

import (
	"github.com/okteto/okteto/pkg/model"
)

const (
	upEvent        = "Up"
	upErrorEvent   = "Up Error"
	syncErrorEvent = "Sync Error"
	reconnectEvent = "Reconnect"
)

const (
	// ReconnectCauseDefault is the default cause for a reconnection
	ReconnectCauseDefault = "unrecognised"

	// ReconnectCauseDevPodRecreated is cause when pods UID change between retrys
	ReconnectCauseDevPodRecreated = "dev-pod-recreated"
)

// TrackUpMetadata defines the properties an up can have
type TrackUpMetadata struct {
	IsV2                   bool
	ManifestType           model.Archetype
	IsInteractive          bool
	IsOktetoRepository     bool
	HasDependenciesSection bool
	HasBuildSection        bool
	HasDeploySection       bool
	Success                bool
	HasReverse             bool
	IsHybridDev            bool
	Mode                   string
}

// TrackUp sends a tracking event to mixpanel when the user activates a development container
func TrackUp(m TrackUpMetadata) {
	props := map[string]interface{}{
		"isInteractive":          m.IsInteractive,
		"isV2":                   m.IsV2,
		"manifestType":           m.ManifestType,
		"isOktetoRepository":     m.IsOktetoRepository,
		"hasDependenciesSection": m.HasDependenciesSection,
		"hasBuildSection":        m.HasBuildSection,
		"hasDeploySection":       m.HasDeploySection,
		"hasReverse":             m.HasReverse,
		"mode":                   m.Mode,
	}
	track(upEvent, m.Success, props)
}

// TrackUpError sends a tracking event to mixpanel when the okteto up command fails
func TrackUpError(success bool) {
	track(upErrorEvent, success, nil)
}

const eventActivateUp = "Up Duration Time"

// TrackSecondsActivateUp sends a eventActivateUp to mixpanel
// measures the duration for command up to be active, from start until first exec is done
func TrackSecondsActivateUp(seconds float64) {
	props := map[string]interface{}{
		"seconds": seconds,
	}
	track(eventActivateUp, true, props)
}

// TrackReconnect sends a tracking event to mixpanel when the development container reconnect
func TrackReconnect(success bool, cause string) {
	props := map[string]interface{}{
		"cause": cause,
	}
	track(reconnectEvent, success, props)
}

// TrackSyncError sends a tracking event to mixpanel when the init sync fails
func TrackSyncError() {
	track(syncErrorEvent, false, nil)
}

const eventSecondsToScanLocalFolders = "Up Scan Local Folders Duration"

// TrackSecondsToScanLocalFolders sends eventSecondsToScanLocalFolders to mixpanel with duration as seconds
func TrackSecondsToScanLocalFolders(seconds float64) {
	props := map[string]interface{}{
		"seconds": seconds,
	}
	track(eventSecondsToScanLocalFolders, true, props)
}

const eventSecondsToSyncContext = "Up Sync Context Duration"

// TrackSecondsToScanLocalFolders sends eventSecondsToScanLocalFolders to mixpanel with duration as seconds
func TrackSecondsToSyncContext(seconds float64) {
	props := map[string]interface{}{
		"seconds": seconds,
	}
	track(eventSecondsToSyncContext, true, props)
}
