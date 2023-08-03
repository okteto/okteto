package analytics

import (
	"time"

	"github.com/okteto/okteto/pkg/model"
)

const (
	// Event that tracks when a user activates a development container
	upEvent           = "Up"
	syncResetDatabase = "Sync Reset Database"
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
	ActivateDuration       time.Duration
	InitialSyncDuration    time.Duration
	IsReconnect            bool
	ReconnectCause         string
	ErrSync                bool
}

func NewUpMetadata() *UpMetadata {
	return &UpMetadata{}
}

func (u *UpMetadata) toProps() map[string]interface{} {
	return map[string]interface{}{
		"isInteractive":              u.IsInteractive,
		"isV2":                       u.IsV2,
		"manifestType":               u.ManifestType,
		"isOktetoRepository":         u.IsOktetoRepository,
		"hasDependenciesSection":     u.HasDependenciesSection,
		"hasBuildSection":            u.HasBuildSection,
		"hasDeploySection":           u.HasDeploySection,
		"hasReverse":                 u.HasReverse,
		"mode":                       u.Mode,
		"failActivate":               u.FailActivate,
		"activateDurationSeconds":    u.ActivateDuration.Seconds(),
		"initialSyncDurationSeconds": u.InitialSyncDuration.Seconds(),
		"isReconnect":                u.IsReconnect,
		"reconnectCause":             u.ReconnectCause,
		"errSync":                    u.ErrSync,
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

func (u *UpMetadata) AddActivateDuration(duration time.Duration) {
	u.ActivateDuration = duration
}

func (u *UpMetadata) AddInitialSyncDuration(duration time.Duration) {
	u.InitialSyncDuration = duration
}

const (
	// ReconnectCauseDefault is the default cause for a reconnection
	ReconnectCauseDefault = "unrecognised"

	// ReconnectCauseDevPodRecreated is cause when pods UID change between retrys
	ReconnectCauseDevPodRecreated = "dev-pod-recreated"
)

func (u *UpMetadata) AddReconnect(cause string) {
	u.IsReconnect = true
	u.ReconnectCause = cause
}

func (u *UpMetadata) AddErrSync() {
	u.ErrSync = true
}

// TrackUp sends a tracking event to mixpanel when the user activates a development container
func (a *AnalyticsTracker) TrackUp(success bool, m *UpMetadata) {
	a.trackFn(upEvent, success, m.toProps())
}

// TrackResetDatabase sends a tracking event to mixpanel when the syncthing database is reset
func TrackResetDatabase(success bool) {
	track(syncResetDatabase, success, nil)
}
