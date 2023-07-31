package analytics

import (
	"time"

	"github.com/okteto/okteto/pkg/model"
)

const deployEvent = "Deploy"

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
