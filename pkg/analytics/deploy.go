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

const deployEvent = "Deploy"

// DeployMetadata contains the metadata of a deploy event
type DeployMetadata struct {
	Err                    error
	PipelineType           model.Archetype
	DeployType             string
	Duration               time.Duration
	Success                bool
	IsOktetoRepo           bool
	IsPreview              bool
	HasDependenciesSection bool
	HasBuildSection        bool
	IsRemote               bool
}

// TrackDeploy sends a tracking event to mixpanel when the user deploys from command okteto deploy
func (a *Tracker) TrackDeploy(metadata DeployMetadata) {
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
	a.trackFn(deployEvent, metadata.Success, props)
}
