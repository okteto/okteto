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
	ErrorReason              string // image_build: bounded error category on failure, never the raw error
	ConnectionType           string // image_build: proxy or legacy
	WaitForBuildkitAvailable time.Duration
	BuildkitDuration         time.Duration
	ContextTransferDuration  time.Duration
	BuildContextHashDuration time.Duration
	CacheHitDuration         time.Duration
	BuildDuration            time.Duration
	CloneDuration            time.Duration
	BuildContextSize         int64 // image_build: build context size in bytes
	CacheHit                 bool
	Success                  bool
}

func NewImageBuildMetadata() *ImageBuildMetadata {
	return &ImageBuildMetadata{}
}

func (m *ImageBuildMetadata) toMixpanelProps() map[string]any {
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

	return props
}

// toProductEventProps returns the properties of the image_build product event.
// Fields with zero/empty values are omitted.
func (m *ImageBuildMetadata) toProductEventProps() map[string]any {
	props := map[string]any{
		"service":  m.Name,
		"result":   m.Success,
		"is_cache": m.CacheHit,
	}
	if d := int(m.BuildDuration.Seconds()); d > 0 {
		props["duration_seconds"] = d
	}
	if d := int(m.WaitForBuildkitAvailable.Seconds()); d > 0 {
		props["queue_duration_seconds"] = d
	}
	if d := int(m.BuildkitDuration.Seconds()); d > 0 {
		props["buildkit_duration_seconds"] = d
	}
	if d := m.ContextTransferDuration.Milliseconds(); d > 0 {
		props["build_context_duration_ms"] = d
	}
	if m.BuildContextSize > 0 {
		props["build_context_size_bytes"] = m.BuildContextSize
	}
	if m.ConnectionType != "" {
		props["connection_type"] = m.ConnectionType
	}
	if m.RepoURL != "" {
		props["repo_url"] = hashString(normalizeRepoURL(m.RepoURL))
	}
	if !m.Success && m.ErrorReason != "" {
		props["error_reason"] = m.ErrorReason
	}
	return props
}

// TrackImageBuild sends the imageBuild and image_build events.
func (a *Tracker) TrackImageBuild(ctx context.Context, m *ImageBuildMetadata) {
	a.trackFn(imageBuildEvent, m.Success, m.toMixpanelProps())
	a.events.track(ctx, productImageBuildEvent, m.toProductEventProps(), a.events.withNamespace(m.Namespace))
}
