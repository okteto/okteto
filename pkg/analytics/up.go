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
)

// UpMetricsMetadata defines the properties of the Up event we want to track
type UpMetricsMetadata struct {
	isV2                   bool
	manifestType           model.Archetype
	isInteractive          bool
	isOktetoRepository     bool
	hasDependenciesSection bool
	hasBuildSection        bool
	hasDeploySection       bool
	hasReverse             bool
	isHybridDev            bool
	mode                   string
	failActivate           bool
	activateDuration       time.Duration
	initialSyncDuration    time.Duration
	isReconnect            bool
	reconnectCause         string
	errSync                bool
	errResetDatabase       bool
	success                bool
}

func NewUpMetricsMetadata() *UpMetricsMetadata {
	return &UpMetricsMetadata{}
}

func (u *UpMetricsMetadata) toProps() map[string]interface{} {
	return map[string]interface{}{
		"isInteractive":              u.isInteractive,
		"isV2":                       u.isV2,
		"manifestType":               u.manifestType,
		"isOktetoRepository":         u.isOktetoRepository,
		"hasDependenciesSection":     u.hasDependenciesSection,
		"hasBuildSection":            u.hasBuildSection,
		"hasDeploySection":           u.hasDeploySection,
		"hasReverse":                 u.hasReverse,
		"mode":                       u.mode,
		"failActivate":               u.failActivate,
		"activateDurationSeconds":    u.activateDuration.Seconds(),
		"initialSyncDurationSeconds": u.initialSyncDuration.Seconds(),
		"isReconnect":                u.isReconnect,
		"reconnectCause":             u.reconnectCause,
		"errSync":                    u.errSync,
		"errResetDatabase":           u.errResetDatabase,
	}
}

func (u *UpMetricsMetadata) AddManifestProps(m *model.Manifest) {
	u.isV2 = m.IsV2
	u.manifestType = m.Type
	u.hasDependenciesSection = m.HasDependenciesSection()
	u.hasBuildSection = m.HasBuildSection()
	u.hasDeploySection = m.HasDeploySection()
}

func (u *UpMetricsMetadata) AddDevProps(d *model.Dev) {
	u.hasReverse = len(d.Reverse) > 0
	u.mode = d.Mode
	u.isInteractive = d.IsInteractive()

}

func (u *UpMetricsMetadata) AddRepositoryProps(isOktetoRepository bool) {
	u.isOktetoRepository = isOktetoRepository
}

func (u *UpMetricsMetadata) SetFailActivate() {
	u.failActivate = true
}

func (u *UpMetricsMetadata) AddActivateDuration(duration time.Duration) {
	u.activateDuration = duration
}

func (u *UpMetricsMetadata) AddInitialSyncDuration(duration time.Duration) {
	u.initialSyncDuration = duration
}

const (
	// ReconnectCauseDefault is the default cause for a reconnection
	ReconnectCauseDefault = "unrecognised"

	// ReconnectCauseDevPodRecreated is cause when pods UID change between retrys
	ReconnectCauseDevPodRecreated = "dev-pod-recreated"
)

func (u *UpMetricsMetadata) AddReconnect(cause string) {
	u.isReconnect = true
	u.reconnectCause = cause
}

func (u *UpMetricsMetadata) AddErrSync() {
	u.errSync = true
}

func (u *UpMetricsMetadata) AddErrResetDatabase() {
	u.errResetDatabase = true
}

func (u *UpMetricsMetadata) CommandSuccess() {
	u.success = true
}

// TrackUp sends a tracking event to mixpanel when the user activates a development container
func (a *AnalyticsTracker) TrackUp(m *UpMetricsMetadata) {
	a.trackFn(upEvent, m.success, m.toProps())
}
