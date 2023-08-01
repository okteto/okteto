package analytics

import (
	"time"

	"github.com/okteto/okteto/pkg/model"
)

const (
	upEvent = "Up"
)

const (
	// ReconnectCauseDefault is the default cause for a reconnection
	ReconnectCauseDefault = "unrecognised"

	// ReconnectCauseDevPodRecreated is cause when pods UID change between retrys
	ReconnectCauseDevPodRecreated = "dev-pod-recreated"
)

// UpMetadata defines the properties an up can have
type UpMetadata struct {
	IsV2                     bool
	ManifestType             model.Archetype
	IsInteractive            bool
	IsOktetoRepository       bool
	HasDependenciesSection   bool
	HasBuildSection          bool
	HasDeploySection         bool
	Success                  bool
	HasReverse               bool
	IsHybridDev              bool
	Mode                     string
	ActivateDuration         time.Duration
	ScanLocalFoldersDuration time.Duration
	SyncContextDuration      time.Duration
	CommandExecutionDuration time.Duration
	ContextConfigDuration    time.Duration
	HasReconnect             bool
	ReconnectCause           string
	Error                    string
	IsSyncError              bool
}

// TrackUp sends a tracking event to mixpanel when the user activates a development container
func (a *AnalyticsTracker) TrackUp(m *UpMetadata) {
	props := map[string]interface{}{
		"isInteractive":                m.IsInteractive,
		"isV2":                         m.IsV2,
		"manifestType":                 m.ManifestType,
		"isOktetoRepository":           m.IsOktetoRepository,
		"hasDependenciesSection":       m.HasDependenciesSection,
		"hasBuildSection":              m.HasBuildSection,
		"hasDeploySection":             m.HasDeploySection,
		"hasReverse":                   m.HasReverse,
		"mode":                         m.Mode,
		"contextConfigDurationSeconds": m.ContextConfigDuration.Seconds(),
		"activateDurationSeconds":      m.ActivateDuration.Seconds(),
		"scanLocalDurationSeconds":     m.ScanLocalFoldersDuration.Seconds(),
		"syncContextDurationSeconds":   m.SyncContextDuration.Seconds(),
		"commandExecDurationSeconds":   m.CommandExecutionDuration.Seconds(),
		"hasReconnect":                 m.HasReconnect,
		"reconnectCause":               m.ReconnectCause,
		"error":                        m.Error,
		"isSyncError":                  m.IsSyncError,
	}
	a.trackFn(upEvent, m.Success, props)
}

// NewUpMetadata returns a default instance for Up Event metadata
func NewUpMetadata(isOktetoRepository bool) *UpMetadata {
	return &UpMetadata{
		IsOktetoRepository: isOktetoRepository,
	}
}

func (m *UpMetadata) TrackMode(devManifest *model.Dev) {
	m.IsInteractive = devManifest.IsInteractive()
	m.HasReverse = len(devManifest.Reverse) > 0
	m.Mode = devManifest.Mode
}

func (m *UpMetadata) TrackManifest(manifest *model.Manifest) {
	m.IsV2 = manifest.IsV2
	m.HasDependenciesSection = manifest.HasDependenciesSection()
	m.HasBuildSection = manifest.HasBuildSection()
	m.HasDeploySection = manifest.HasDeploySection()
	m.ManifestType = manifest.Type
}

// TrackUpError sends a tracking event to mixpanel when the okteto up command fails
func (m *UpMetadata) TrackUpError(err error) {
	m.Error = err.Error()
}

// TrackReconnect sends a tracking event to mixpanel when the development container reconnect
func (m *UpMetadata) TrackReconnect(cause string) {
	m.HasReconnect = true
	m.ReconnectCause = cause
}

// TrackSyncError sends a tracking event to mixpanel when the init sync fails
func (m *UpMetadata) TrackSyncError(err error) {
	m.IsSyncError = true
	m.Error = err.Error()
}

// TrackSecondsToScanLocalFolders sends eventSecondsToScanLocalFolders to mixpanel with duration as seconds
func (m *UpMetadata) TrackScanLocalFoldersDuration(duration time.Duration) {
	m.ScanLocalFoldersDuration = duration
}

// TrackSecondsToScanLocalFolders sends eventSecondsToScanLocalFolders to mixpanel with duration as seconds
func (m *UpMetadata) TrackActivateDuration(duration time.Duration) {
	m.ActivateDuration = duration
}

// TrackSecondsToScanLocalFolders sends eventSecondsToScanLocalFolders to mixpanel with duration as seconds
func (m *UpMetadata) TrackSecondsToSyncContext(duration time.Duration) {
	m.SyncContextDuration = duration
}

// TrackUpTotalCommandExecution sends eventSecondsUpCommandExecution to mixpanel with duration as seconds
func (m *UpMetadata) TrackSecondsUpCommandExecution(duration time.Duration) {
	m.CommandExecutionDuration = duration
}

// TrackUpTotalCommandExecution sends eventSecondsUpCommandExecution to mixpanel with duration as seconds
func (m *UpMetadata) TrackSecondsUpOktetoContextConfig(duration time.Duration) {
	m.ContextConfigDuration = duration
}

func (m *UpMetadata) TrackSuccess(success bool) {
	m.Success = success
}
