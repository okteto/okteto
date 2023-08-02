package analytics

import (
	"time"

	"github.com/okteto/okteto/pkg/model"
)

const (
	// Event that tracks when a user activates a development container
	upEvent                  = "Up"
	durationActivateUpEvent  = "Up Duration Time"
	durationInitialSyncEvent = "Initial Sync Duration Time"
	reconnectEvent           = "Reconnect"
	syncErrorEvent           = "Sync Error"
	syncResetDatabase        = "Sync Reset Database"
)

// UpMetadata defines the properties an up can have
type UpMetadata struct {
	IsV2                   bool
	ManifestType           model.Archetype
	IsInteractive          bool
	IsOktetoRepository     bool
	HasDependenciesSection bool
	HasBuildSection        bool
	HasDeploySection       bool
	HasReverse             bool
	IsHybridDev            bool
	Mode                   string
	FailActivate           bool
}

func NewUpMetadata() *UpMetadata {
	return &UpMetadata{}
}

func (u *UpMetadata) toProps() map[string]interface{} {
	return map[string]interface{}{
		"isInteractive":          u.IsInteractive,
		"isV2":                   u.IsV2,
		"manifestType":           u.ManifestType,
		"isOktetoRepository":     u.IsOktetoRepository,
		"hasDependenciesSection": u.HasDependenciesSection,
		"hasBuildSection":        u.HasBuildSection,
		"hasDeploySection":       u.HasDeploySection,
		"hasReverse":             u.HasReverse,
		"mode":                   u.Mode,
		"failActivate":           u.FailActivate,
	}
}

func (u *UpMetadata) AddManifestProps(m *model.Manifest) {
	u.IsV2 = m.IsV2
	u.ManifestType = m.Type
	u.HasDependenciesSection = m.HasDependenciesSection()
	u.HasBuildSection = m.HasBuildSection()
	u.HasDeploySection = m.HasDeploySection()
}

func (u *UpMetadata) AddDevProps(d *model.Dev) {
	u.HasReverse = len(d.Reverse) > 0
	u.Mode = d.Mode
	u.IsInteractive = d.IsInteractive()

}

func (u *UpMetadata) AddRepositoryProps(isOktetoRepository bool) {
	u.IsOktetoRepository = isOktetoRepository
}

func (u *UpMetadata) SetFailActivate() {
	u.FailActivate = true
}

// TrackUp sends a tracking event to mixpanel when the user activates a development container
func (a *AnalyticsTracker) TrackUp(success bool, m *UpMetadata) {
	a.trackFn(upEvent, success, m.toProps())
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
