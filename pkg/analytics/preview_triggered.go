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

// toProductEventProps returns the properties of the deploy_preview_triggered product event.
func (m DeployPreviewTriggeredMetadata) toProductEventProps() map[string]any {
	props := map[string]any{
		"workflow_id": m.WorkflowID,
		"is_redeploy": m.IsRedeploy,
	}
	if m.ParentWorkflowID != "" {
		props["parent_workflow_id"] = m.ParentWorkflowID
	}
	if m.RepoURL != "" {
		props["repo_url"] = hashString(normalizeRepoURL(m.RepoURL))
	}
	return props
}

// TrackDeployPreviewTriggered sends a deploy_preview_triggered event. The preview
// property is the preview namespace UID, never its name.
func (a *Tracker) TrackDeployPreviewTriggered(ctx context.Context, m DeployPreviewTriggeredMetadata) {
	a.events.track(ctx, productDeployPreviewTriggeredEvent, m.toProductEventProps(), a.events.withPreview(m.Preview))
}
