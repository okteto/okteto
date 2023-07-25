package analytics

import (
	"time"

	"github.com/okteto/okteto/pkg/model"
)

const deployEvent = "Deploy"

type AnalyticsTracker struct {
	TrackFn func(event string, success bool, props map[string]interface{})
}

func NewAnalyticsTracker() *AnalyticsTracker {
	return &AnalyticsTracker{
		TrackFn: track,
	}
}

// DeployMetadata contains the metadata of a deploy event
type DeployMetadata struct {
	Success                bool
	IsOktetoRepo           bool
	Err                    error
	Duration               time.Duration
	PipelineType           model.Archetype
	DeployType             string
	IsPreview              bool
	HasDependenciesSection bool
	HasBuildSection        bool
	IsRemote               bool
}

// TrackDeploy sends a tracking event to mixpanel when the user deploys from command okteto deploy
func (a *AnalyticsTracker) TrackDeploy(metadata DeployMetadata) {
	if metadata.PipelineType == "" {
		metadata.PipelineType = "pipeline"
	}
	props := map[string]any{
		"pipelineType":           metadata.PipelineType,
		"isOktetoRepository":     metadata.IsOktetoRepo,
		"duration":               metadata.Duration.Seconds(),
		"deployType":             metadata.DeployType,
		"isPreview":              metadata.IsPreview,
		"hasDependenciesSection": metadata.HasDependenciesSection,
		"hasBuildSection":        metadata.HasBuildSection,
		"isRemote":               metadata.IsRemote,
	}
	if metadata.Err != nil {
		props["error"] = metadata.Err.Error()
	}
	a.TrackFn(deployEvent, metadata.Success, props)
}

// DestroyMetadata contains the metadata of a destroy event
type DestroyMetadata struct {
	Success      bool
	IsDestroyAll bool
	IsRemote     bool
}

// TrackDestroy sends a tracking event to mixpanel when the user destroys a pipeline from local
func (a *AnalyticsTracker) TrackDestroy(metadata DestroyMetadata) {
	props := map[string]any{
		"isDestroyAll": metadata.IsDestroyAll,
		"isRemote":     metadata.IsRemote,
	}
	a.TrackFn(destroyEvent, metadata.Success, props)
}
