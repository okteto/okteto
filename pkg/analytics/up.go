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
	reconnectEvent           = "Reconnect"
	syncErrorEvent           = "Sync Error"
	syncResetDatabase        = "Sync Reset Database"
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
func (a *AnalyticsTracker) TrackUp(m TrackUpMetadata) {
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
	a.trackFn(upEvent, m.Success, props)
}

// TrackUpError sends a tracking event to mixpanel when the okteto up command fails
func (a *AnalyticsTracker) TrackUpError(success bool) {
	a.trackFn(upErrorEvent, success, nil)
}

const (
	// ReconnectCauseDefault is the default cause for a reconnection
	ReconnectCauseDefault = "unrecognised"

	// ReconnectCauseDevPodRecreated is cause when pods UID change between retrys
	ReconnectCauseDevPodRecreated = "dev-pod-recreated"
)

// TrackReconnect sends a tracking event to mixpanel when the development container reconnect
func (a *AnalyticsTracker) TrackReconnect(success bool, cause string) {
	props := map[string]interface{}{
		"cause": cause,
	}
	a.trackFn(reconnectEvent, success, props)
}

// TrackSyncError sends a tracking event to mixpanel when the init sync fails
func (a *AnalyticsTracker) TrackSyncError() {
	a.trackFn(syncErrorEvent, false, nil)
}

// TrackDurationInitialSync sends a tracking event to mixpanel with initial sync duration
func (a *AnalyticsTracker) TrackDurationInitialSync(durationInitialSync time.Duration) {
	props := map[string]interface{}{
		"duration": durationInitialSync,
	}
	a.trackFn(durationInitialSyncEvent, true, props)
}

// TrackDurationActivateUp sends a tracking event to mixpanel of the time that has elapsed in the execution of up
func (a *AnalyticsTracker) TrackDurationActivateUp(durationActivateUp time.Duration) {
	props := map[string]interface{}{
		"duration": durationActivateUp,
	}
	a.trackFn(durationActivateUpEvent, true, props)
}

// TrackResetDatabase sends a tracking event to mixpanel when the syncthing database is reset
func TrackResetDatabase(success bool) {
	track(syncResetDatabase, success, nil)
}
