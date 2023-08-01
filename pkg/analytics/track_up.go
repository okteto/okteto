package analytics

import (
	"time"

	"github.com/okteto/okteto/pkg/model"
)

const (
	upEvent                  = "Up"
	upErrorEvent             = "Up Error"
	durationActivateUpEvent  = "Up Duration Time"
	durationInitialSyncEvent = "Initial Sync Duration Time"
	syncErrorEvent           = "Sync Error"
	reconnectEvent           = "Reconnect"
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

// TrackDurationActivateUp sends a tracking event to mixpanel of the time that has elapsed in the execution of up
func TrackDurationActivateUp(durationActivateUp time.Duration) {
	props := map[string]interface{}{
		"duration": durationActivateUp,
	}
	track(durationActivateUpEvent, true, props)
}

// TrackDurationInitialSync sends a tracking event to mixpanel with initial sync duration
func TrackDurationInitialSync(durationInitialSync time.Duration) {
	props := map[string]interface{}{
		"duration": durationInitialSync,
	}
	track(durationInitialSyncEvent, true, props)
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


