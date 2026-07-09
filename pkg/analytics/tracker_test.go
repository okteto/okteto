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

import "context"

type mockEvent struct {
	props   map[string]any
	event   string
	success bool
}

type mockAnalyticsBackend struct {
	trackImageBuildFn    func(ctx context.Context, meta *ImageBuildMetadata)
	trackUpFn            func(meta *UpMetricsMetadata)
	trackUpStartedFn     func(service, namespace, repoURL, workflowID string)
	trackDeployStartedFn func(meta DeployStartedMetadata)
	trackDeployFn        func(meta DeployMetadata)
}

func (m *mockAnalyticsBackend) TrackImageBuild(ctx context.Context, meta *ImageBuildMetadata) {
	if m.trackImageBuildFn != nil {
		m.trackImageBuildFn(ctx, meta)
	}
}

func (m *mockAnalyticsBackend) TrackDeployPipelineTriggered(_ context.Context, _ DeployPipelineTriggeredMetadata) {
}

func (m *mockAnalyticsBackend) TrackDeployPreviewTriggered(_ context.Context, _ DeployPreviewTriggeredMetadata) {
}

func (m *mockAnalyticsBackend) TrackWakeTriggered(_ context.Context, _ WakeTriggeredMetadata) {
}

func (m *mockAnalyticsBackend) TrackUp(meta *UpMetricsMetadata) {
	if m.trackUpFn != nil {
		m.trackUpFn(meta)
	}
}

func (m *mockAnalyticsBackend) TrackUpStarted(service, namespace, repoURL, workflowID string) {
	if m.trackUpStartedFn != nil {
		m.trackUpStartedFn(service, namespace, repoURL, workflowID)
	}
}

func (m *mockAnalyticsBackend) TrackDeployStarted(meta DeployStartedMetadata) {
	if m.trackDeployStartedFn != nil {
		m.trackDeployStartedFn(meta)
	}
}

func (m *mockAnalyticsBackend) TrackDeploy(meta DeployMetadata) {
	if m.trackDeployFn != nil {
		m.trackDeployFn(meta)
	}
}
