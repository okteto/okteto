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
	"time"

	"github.com/google/uuid"
	"github.com/okteto/okteto/pkg/model"
)

const (
	// Event that tracks when a user activates a development container
	upEvent = "Up"

	// reconnectCauseDefault is the default cause for a reconnection
	reconnectCauseDefault = "unrecognised"

	// reconnectCauseDevPodRecreated is cause when pods UID change between retrys
	reconnectCauseDevPodRecreated = "dev-pod-recreated"
)

// UpMetricsMetadata defines the properties of the Up event we want to track
type UpMetricsMetadata struct {
	workflowID     string
	manifestType   model.Archetype
	mode           string
	reconnectCause string
	service        string
	namespace      string
	repoURL        string

	activateDuration             time.Duration
	initialSyncDuration          time.Duration
	oktetoCtxConfigDuration      time.Duration
	devContainerCreationDuration time.Duration
	contextSyncDuration          time.Duration
	localFoldersScanDuration     time.Duration
	execDuration                 time.Duration

	reconnectCount int

	isInteractive            bool
	isOktetoRepository       bool
	hasDependenciesSection   bool
	hasBuildSection          bool
	hasDeploySection         bool
	hasReverse               bool
	isHybridDev              bool
	failActivate             bool
	isReconnect              bool
	isBuildExecuted          bool
	errSync                  bool
	errSyncResetDatabase     bool
	errSyncInsufficientSpace bool
	errSyncLostSyncthing     bool
	success                  bool
	hasRunDeploy             bool
	isAutoDownEnabled        bool
}

// NewUpMetricsMetadata returns a new UpMetricsMetadata with a unique workflow ID.
func NewUpMetricsMetadata() *UpMetricsMetadata {
	return &UpMetricsMetadata{
		workflowID: uuid.New().String(),
	}
}

// WorkflowID returns the unique ID that correlates up_started and up events for this session.
func (u *UpMetricsMetadata) WorkflowID() string {
	return u.workflowID
}

// toProps transforms UpMetricsMetadata into a map to be able to send it to mixpanel
func (u *UpMetricsMetadata) toProps() map[string]any {
	return map[string]any{
		"isInteractive":                       u.isInteractive,
		"manifestType":                        u.manifestType,
		"isOktetoRepository":                  u.isOktetoRepository,
		"hasDependenciesSection":              u.hasDependenciesSection,
		"hasBuildSection":                     u.hasBuildSection,
		"hasDeploySection":                    u.hasDeploySection,
		"hasReverse":                          u.hasReverse,
		"mode":                                u.mode,
		"failActivate":                        u.failActivate,
		"activateDurationSeconds":             u.activateDuration.Seconds(),
		"initialSyncDurationSeconds":          u.initialSyncDuration.Seconds(),
		"isReconnect":                         u.isReconnect,
		"reconnectCause":                      u.reconnectCause,
		"errSync":                             u.errSync,
		"errSyncResetDatabase":                u.errSyncResetDatabase,
		"errSyncInsufficientSpace":            u.errSyncInsufficientSpace,
		"errSyncLostSyncthing":                u.errSyncLostSyncthing,
		"hasRunDeploy":                        u.hasRunDeploy,
		"oktetoCtxConfigDurationSeconds":      u.oktetoCtxConfigDuration.Seconds(),
		"devContainerCreationDurationSeconds": u.devContainerCreationDuration.Seconds(),
		"contextSyncDurationSeconds":          u.contextSyncDuration.Seconds(),
		"localFoldersScanDurationSeconds":     u.localFoldersScanDuration.Seconds(),
		"execDurationSeconds":                 u.execDuration.Seconds(),
		"isAutoDownEnabled":                   u.isAutoDownEnabled,
	}
}

// ManifestProps adds the tracking properties of the repository manifest
func (u *UpMetricsMetadata) ManifestProps(m *model.Manifest) {
	u.manifestType = m.Type
	u.hasDependenciesSection = m.HasDependenciesSection()
	u.hasBuildSection = m.HasBuildSection()
	u.hasDeploySection = m.HasDeploySection()
}

// DevProps adds the tracking properties of the service development manifest
func (u *UpMetricsMetadata) DevProps(d *model.Dev) {
	u.hasReverse = len(d.Reverse) > 0
	u.mode = d.Mode
	u.isInteractive = d.IsInteractive()
	u.service = d.Name
}

// RepositoryProps adds the tracking properties of the repository
func (u *UpMetricsMetadata) RepositoryProps(isOktetoRepository bool) {
	u.isOktetoRepository = isOktetoRepository
}

// FailActivate sets to true the property failActivate
func (u *UpMetricsMetadata) FailActivate() {
	u.failActivate = true
}

// ActivateDuration adds the duration of up activation
func (u *UpMetricsMetadata) ActivateDuration(duration time.Duration) {
	u.activateDuration = duration
}

// InitialSyncDuration adds the duration of the initial sync
func (u *UpMetricsMetadata) InitialSyncDuration(duration time.Duration) {
	u.initialSyncDuration = duration
}

// ReconnectDefault sets to true the property isReconnect and adds the cause "unrecognised"
func (u *UpMetricsMetadata) ReconnectDefault() {
	u.isReconnect = true
	u.reconnectCause = reconnectCauseDefault
	u.reconnectCount++
}

// ReconnectDevPodRecreated sets to true the property isReconnect and adds the cause "dev-pod-recreated"
func (u *UpMetricsMetadata) ReconnectDevPodRecreated() {
	u.isReconnect = true
	u.reconnectCause = reconnectCauseDevPodRecreated
	u.reconnectCount++
}

// ErrSync sets to true the property errSync
func (u *UpMetricsMetadata) ErrSync() {
	u.errSync = true
}

// ErrSyncResetDatabase sets to true the property errResetDatabase
func (u *UpMetricsMetadata) ErrSyncResetDatabase() {
	u.errSyncResetDatabase = true
}

// ErrSyncInsufficientSpace sets to true the property errResetDatabase
func (u *UpMetricsMetadata) ErrSyncInsufficientSpace() {
	u.errSyncInsufficientSpace = true
}

// ErrSyncLostSyncthing sets to true the property errResetDatabase
func (u *UpMetricsMetadata) ErrSyncLostSyncthing() {
	u.errSyncLostSyncthing = true
}

// CommandSuccess sets to true the property success
func (u *UpMetricsMetadata) CommandSuccess() {
	u.success = true
}

func (u *UpMetricsMetadata) HasRunDeploy() {
	u.hasRunDeploy = true
}

// HasRunBuild marks that a build was executed during this up session.
func (u *UpMetricsMetadata) HasRunBuild() {
	u.isBuildExecuted = true
}

// IsBuildExecuted reports whether a build ran during this up session.
func (u *UpMetricsMetadata) IsBuildExecuted() bool {
	return u.isBuildExecuted
}

// SetRepoURL records the git remote origin URL for the session.
func (u *UpMetricsMetadata) SetRepoURL(rawURL string) {
	u.repoURL = rawURL
}

// SetNamespace records the namespace for the session.
func (u *UpMetricsMetadata) SetNamespace(namespace string) {
	u.namespace = namespace
}

func (u *UpMetricsMetadata) OktetoContextConfig(duration time.Duration) {
	u.oktetoCtxConfigDuration = duration
}

func (u *UpMetricsMetadata) DevContainerCreation(duration time.Duration) {
	u.devContainerCreationDuration = duration
}

func (u *UpMetricsMetadata) ContextSync(duration time.Duration) {
	u.contextSyncDuration = duration
}

func (u *UpMetricsMetadata) LocalFolderScan(duration time.Duration) {
	u.localFoldersScanDuration = duration
}

func (u *UpMetricsMetadata) ExecDuration(duration time.Duration) {
	u.execDuration = duration
}

func (u *UpMetricsMetadata) IsAutoDownEnabled(enabled bool) {
	u.isAutoDownEnabled = enabled
}

func (u *UpMetricsMetadata) errorReason() string {
	switch {
	case u.failActivate:
		return "fail_activate"
	case u.errSyncInsufficientSpace:
		return "err_sync_insufficient_space"
	case u.errSyncResetDatabase:
		return "err_sync_reset_database"
	case u.errSyncLostSyncthing:
		return "err_sync_lost_syncthing"
	case u.errSync:
		return "err_sync"
	default:
		return ""
	}
}

func (u *UpMetricsMetadata) toPostHogProps() map[string]any {
	props := map[string]any{
		"result":             u.success,
		"manifest_type":      string(u.manifestType),
		"is_interactive":     u.isInteractive,
		"is_build_executed":  u.isBuildExecuted,
		"is_deploy_executed": u.hasRunDeploy,
		"has_build_section":  u.hasBuildSection,
		"has_deploy_section": u.hasDeploySection,
		"is_reconnect":       u.isReconnect,
		"reconnect_count":    u.reconnectCount,
		"is_auto_down":       u.isAutoDownEnabled,
	}
	repoURL := ""
	if u.repoURL != "" {
		repoURL = hashString(normalizeRepoURL(u.repoURL))
	}
	props["workflow_id"] = u.workflowID
	props["service"] = u.service
	props["repo_url"] = repoURL
	props["duration_seconds"] = int(u.execDuration.Seconds())
	props["initial_sync_duration_seconds"] = int(u.initialSyncDuration.Seconds())
	props["dev_container_creation_duration_seconds"] = int(u.devContainerCreationDuration.Seconds())
	if u.isReconnect && u.reconnectCause != "" {
		props["reconnect_cause"] = u.reconnectCause
	}
	if !u.success {
		if reason := u.errorReason(); reason != "" {
			props["error_reason"] = reason
		}
	}
	return props
}

// TrackUp sends a tracking event when the user activates a development container.
// Mixpanel receives the event via trackFn; PostHog via the backends slice.
func (a *Tracker) TrackUp(m *UpMetricsMetadata) {
	a.trackFn(upEvent, m.success, m.toProps())
	for _, b := range a.backends {
		b.TrackUp(m)
	}
}

// TrackUpStarted fires the okteto_up_started event at the beginning of the up command.
func (a *Tracker) TrackUpStarted(service, namespace, repoURL, workflowID string) {
	for _, b := range a.backends {
		b.TrackUpStarted(service, namespace, repoURL, workflowID)
	}
}
