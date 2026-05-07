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
	ErrorCategory            string // PostHog: error category, empty on success
	ConnectionType           string // PostHog: proxy or legacy
	WaitForBuildkitAvailable time.Duration
	BuildkitDuration         time.Duration
	ContextTransferDuration  time.Duration
	BuildContextHashDuration time.Duration
	CacheHitDuration         time.Duration
	BuildDuration            time.Duration
	CloneDuration            time.Duration
	BuildContextSize         int64 // PostHog: build context size in bytes
	CacheHit                 bool
	Success                  bool
}

func NewImageBuildMetadata() *ImageBuildMetadata {
	return &ImageBuildMetadata{}
}

func (m *ImageBuildMetadata) toMixpanelProps() map[string]interface{} {
	props := map[string]interface{}{
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
		props["repoURL"] = hashString(m.RepoURL)
	}

	return props
}

// toPostHogProps returns the PostHog-specific property map for an image_build event.
// errorCategory is omitted on success.
func (m *ImageBuildMetadata) toPostHogProps() map[string]interface{} {
	props := map[string]interface{}{
		"service":                        m.Name,
		"duration_seconds":               int(m.BuildDuration.Seconds()),
		"queue_duration_seconds":         int(m.WaitForBuildkitAvailable.Seconds()),
		"buildkit_duration_seconds":      int(m.BuildkitDuration.Seconds()),
		"build_context_duration_seconds": int(m.ContextTransferDuration.Seconds()),
		"result":                         m.Success,
		"build_context_size_bytes":       m.BuildContextSize,
		"is_cache":                       m.CacheHit,
		"connection_type":                m.ConnectionType,
		"repo_url":                       m.RepoURL,
	}
	if !m.Success {
		props["errorCategory"] = m.ErrorCategory
	}
	return props
}

// TrackImageBuild sends an image build event to all registered backends.
func (a *Tracker) TrackImageBuild(ctx context.Context, m *ImageBuildMetadata) {
	for _, b := range a.backends {
		b.TrackImageBuild(ctx, m)
	}
}
