// Copyright 2023-2025 The Okteto Authors
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

import "context"

// DeployPipelineTriggeredMetadata contains the metadata of a deploy_pipeline_triggered event
type DeployPipelineTriggeredMetadata struct {
	WorkflowID       string
	ParentWorkflowID string
	RepoURL          string
	Namespace        string
	DeployType       string
	UIElement        string
	IsWithinPreview  bool
	IsRedeploy       bool
}

// TrackDeployPipelineTriggered sends a deploy_pipeline_triggered event to all registered backends.
func (a *Tracker) TrackDeployPipelineTriggered(ctx context.Context, m DeployPipelineTriggeredMetadata) {
	for _, b := range a.backends {
		b.TrackDeployPipelineTriggered(ctx, m)
	}
}
