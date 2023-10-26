package analytics

import (
	"time"
)

const (
	imageBuildEvent = "imageBuild"
)

type ImageBuildMetadata struct {
	Name                     string
	RepoURL                  string
	RepoHash                 string
	RepoHashDuration         time.Duration
	BuildContextHash         string
	BuildContextHashDuration time.Duration
	CacheHit                 bool
	CacheHitDuration         time.Duration
	BuildDuration            time.Duration
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
	}

	if m.Name != "" {
		props["name"] = hashString(m.Name)
	}
	if m.RepoURL != "" {
		props["repoURL"] = hashString(m.RepoURL)
	}

	return props
}

func (a *AnalyticsTracker) TrackImageBuild(metaList ...*ImageBuildMetadata) {
	for _, m := range metaList {
		a.trackFn(imageBuildEvent, m.Success, m.toProps())
	}
}
