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

// mixpanelBackend forwards image build events to the existing Mixpanel track() function.
// It is a thin wrapper — no Mixpanel behaviour changes.
type mixpanelBackend struct {
	trackFn func(event string, success bool, props map[string]any)
}

func newMixpanelBackend() *mixpanelBackend {
	return &mixpanelBackend{trackFn: track}
}

func (b *mixpanelBackend) TrackImageBuild(_ context.Context, m *ImageBuildMetadata) {
	b.trackFn(imageBuildEvent, m.Success, m.toMixpanelProps())
}

func (b *mixpanelBackend) TrackDeployPipelineTriggered(_ context.Context, _ DeployPipelineTriggeredMetadata) {
}

func (b *mixpanelBackend) TrackDeployPreviewTriggered(_ context.Context, _ DeployPreviewTriggeredMetadata) {
}

func (b *mixpanelBackend) TrackWakeTriggered(_ context.Context, _ WakeTriggeredMetadata) {
}

func (b *mixpanelBackend) TrackUp(_ *UpMetricsMetadata) {}

func (b *mixpanelBackend) TrackUpStarted(_, _, _, _ string) {}

func (b *mixpanelBackend) TrackDeployStarted(_ DeployStartedMetadata) {}

func (b *mixpanelBackend) TrackDeploy(_ DeployMetadata) {}
