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
	RepoHash                 string
	BuildContextHash         string
	Initiator                string
	RepoHashDuration         time.Duration
	BuildContextHashDuration time.Duration
	CacheHitDuration         time.Duration
	BuildDuration            time.Duration
	CacheHit                 bool
	Success                  bool
}

func NewImageBuildMetadata() *ImageBuildMetadata {
	return &ImageBuildMetadata{}
}

func (m *ImageBuildMetadata) toProps() map[string]interface{} {
	props := map[string]interface{}{
		"name":                            m.Name,
		"repoURL":                         m.RepoURL,
		"repoHash":                        m.RepoHash,
		"repoHashDurationSeconds":         m.RepoHashDuration.Seconds(),
		"cacheHit":                        m.CacheHit,
		"cacheHitDurationSeconds":         m.CacheHitDuration.Seconds(),
		"buildDurationSeconds":            m.BuildDuration.Seconds(),
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

func (a *Tracker) TrackImageBuild(_ context.Context, m *ImageBuildMetadata) {
	a.trackFn(imageBuildEvent, m.Success, m.toProps())
}
