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
	"context"
	"time"
)

const (
	imageBuildEvent = "imageBuild"
)

type ImageBuildMetadata struct {
	Name                     string
	Namespace                string
	DevenvName               string
	RepoURL                  string
	BuildContextHash         string
	Initiator                string
	ErrorReason              string // bounded error category on failure, never the raw error
	ConnectionType           string // proxy or legacy
	WaitForBuildkitAvailable time.Duration
	BuildkitDuration         time.Duration
	ContextTransferDuration  time.Duration
	BuildContextHashDuration time.Duration
	CacheHitDuration         time.Duration
	BuildDuration            time.Duration
	CloneDuration            time.Duration
	BuildContextSize         int64 // build context size in bytes
	CacheHit                 bool
	Success                  bool
}

func NewImageBuildMetadata() *ImageBuildMetadata {
	return &ImageBuildMetadata{}
}

// toProps returns the properties of the imageBuild event: the Mixpanel ones
// plus the ones that PostHog's image_build had and imageBuild didn't.
// The ported ones with zero/empty values are omitted.
func (m *ImageBuildMetadata) toProps() map[string]any {
	props := map[string]any{
		"name":                            m.Name,
		"repoURL":                         m.RepoURL,
		"waitForBuildkitAvailable":        m.WaitForBuildkitAvailable.Seconds(),
		"cacheHit":                        m.CacheHit,
		"cacheHitDurationSeconds":         m.CacheHitDuration.Seconds(),
		"buildDurationSeconds":            m.BuildDuration.Seconds(),
		"cloneDurationSeconds":            m.CloneDuration.Seconds(),
		"buildContextHash":                m.BuildContextHash,
		"buildContextHashDurationSeconds": m.BuildContextHashDuration.Seconds(),
		"initiator":                       m.Initiator,
	}

	if m.Name != "" {
		props["name"] = hashString(m.Name)
	}
	if m.RepoURL != "" {
		props["repoURL"] = hashString(normalizeRepoURL(m.RepoURL))
	}
	if d := int(m.BuildkitDuration.Seconds()); d > 0 {
		props["buildkitDurationSeconds"] = d
	}
	if d := m.ContextTransferDuration.Milliseconds(); d > 0 {
		props["buildContextDurationMs"] = d
	}
	if m.BuildContextSize > 0 {
		props["buildContextSizeBytes"] = m.BuildContextSize
	}
	if m.ConnectionType != "" {
		props["connectionType"] = m.ConnectionType
	}
	if !m.Success && m.ErrorReason != "" {
		props["errorReason"] = m.ErrorReason
	}
	return props
}

// TrackImageBuild sends the imageBuild event.
func (a *Tracker) TrackImageBuild(ctx context.Context, m *ImageBuildMetadata) {
	a.events.trackWithProductProperties(ctx, imageBuildEvent, m.Success, m.toProps(), a.events.withNamespace(m.Namespace))
}
