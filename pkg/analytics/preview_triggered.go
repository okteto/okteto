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

// DeployPreviewTriggeredMetadata contains the metadata of a deploy_preview_triggered event
type DeployPreviewTriggeredMetadata struct {
	WorkflowID       string
	ParentWorkflowID string
	RepoURL          string
	Preview          string
	UIElement        string
	IsRedeploy       bool
}

// TrackDeployPreviewTriggered sends a deploy_preview_triggered event to all registered backends.
func (a *Tracker) TrackDeployPreviewTriggered(ctx context.Context, m DeployPreviewTriggeredMetadata) {
	for _, b := range a.backends {
		b.TrackDeployPreviewTriggered(ctx, m)
	}
}
