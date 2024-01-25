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
	manifestType   model.Archetype
	mode           string
	reconnectCause string

	activateDuration             time.Duration
	initialSyncDuration          time.Duration
	oktetoCtxConfigDuration      time.Duration
	devContainerCreationDuration time.Duration
	contextSyncDuration          time.Duration
	localFoldersScanDuration     time.Duration
	execDuration                 time.Duration

	isV2                     bool
	isInteractive            bool
	isOktetoRepository       bool
	hasDependenciesSection   bool
	hasBuildSection          bool
	hasDeploySection         bool
	hasReverse               bool
	isHybridDev              bool
	failActivate             bool
	isReconnect              bool
	errSync                  bool
	errSyncResetDatabase     bool
	errSyncInsufficientSpace bool
	errSyncLostSyncthing     bool
	success                  bool
	hasRunDeploy             bool
}

// NewUpMetricsMetadata returns an empty instance of UpMetricsMetadata
func NewUpMetricsMetadata() *UpMetricsMetadata {
	return &UpMetricsMetadata{}
}

// toProps transforms UpMetricsMetadata into a map to be able to send it to mixpanel
func (u *UpMetricsMetadata) toProps() map[string]interface{} {
	return map[string]interface{}{
		"isInteractive":                       u.isInteractive,
		"isV2":                                u.isV2,
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
	}
}

// ManifestProps adds the tracking properties of the repository manifest
func (u *UpMetricsMetadata) ManifestProps(m *model.Manifest) {
	u.isV2 = m.IsV2
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
}

// ReconnectDevPodRecreated sets to true the property isReconnect and adds the cause "dev-pod-recreated"
func (u *UpMetricsMetadata) ReconnectDevPodRecreated() {
	u.isReconnect = true
	u.reconnectCause = reconnectCauseDevPodRecreated
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

// TrackUp sends a tracking event to mixpanel when the user activates a development container
func (a *Tracker) TrackUp(m *UpMetricsMetadata) {
	a.trackFn(upEvent, m.success, m.toProps())
}
