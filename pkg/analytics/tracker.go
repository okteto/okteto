// Copyright 2026 The Okteto Authors
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

// analyticsBackend is implemented by each analytics provider.
// Add a new method here when an event gains PostHog coverage.
type analyticsBackend interface {
	TrackImageBuild(ctx context.Context, meta *ImageBuildMetadata)
	TrackDeployPipelineTriggered(ctx context.Context, m DeployPipelineTriggeredMetadata)
	TrackDeployPreviewTriggered(ctx context.Context, m DeployPreviewTriggeredMetadata)
	TrackWakeTriggered(ctx context.Context, m WakeTriggeredMetadata)
	TrackUp(meta *UpMetricsMetadata)
	TrackUpStarted(service, namespace, repoURL, workflowID string)
	TrackDeployStarted(meta DeployStartedMetadata)
	TrackDeploy(meta DeployMetadata)
}

// closer is implemented by backends that hold resources that need flushing on exit.
type closer interface {
	Close()
}

// Tracker dispatches analytics events to all registered backends.
type Tracker struct {
	trackFn  func(event string, success bool, props map[string]any)
	backends []analyticsBackend
}

// NewAnalyticsTracker creates a Tracker wired to all active backends.
func NewAnalyticsTracker() *Tracker {
	return &Tracker{
		trackFn: track,
		backends: []analyticsBackend{
			newMixpanelBackend(),
			newPostHogBackend(),
		},
	}
}

// Close flushes and shuts down every backend that holds resources.
// Call this once before the process exits (e.g. defer at.Close() in main).
func (t *Tracker) Close() {
	for _, b := range t.backends {
		if c, ok := b.(closer); ok {
			c.Close()
		}
	}
}
