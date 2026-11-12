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

import (
	"context"
	"errors"
	"testing"
	"time"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// hashedTestRepoURL is the SHA-256 of the normalized https://github.com/org/repo.
const hashedTestRepoURL = "bdb72e6e68b80f9ed3bbdb0ad1d2f8b4fac8ade379eb82182de40a3357a2d3b3"

type mockEvent struct {
	props   map[string]any
	event   string
	success bool
}

// newTestTracker returns a Tracker that records the track() events in the
// returned slice and sends the product events to fake.
func newTestTracker(fake *fakeEventTracker, resolver NamespaceUIDResolver) (*Tracker, *[]mockEvent) {
	tracked := &[]mockEvent{}
	return &Tracker{
		trackFn: func(event string, success bool, props map[string]any) {
			*tracked = append(*tracked, mockEvent{event: event, success: success, props: props})
		},
		events: newTestEventClient(fake, resolver),
	}, tracked
}

// sendOne waits for the pending product events and returns the only one sent.
func sendOne(t *testing.T, tracker *Tracker, fake *fakeEventTracker) sentEvent {
	t.Helper()
	tracker.Close()
	events := fake.sent()
	require.Len(t, events, 1)
	return events[0]
}

func TestTracker_NilEventsClient(t *testing.T) {
	setupAnalyticsContext(t, true)
	tracker := &Tracker{trackFn: func(string, bool, map[string]any) {}}

	require.NotPanics(t, func() {
		tracker.TrackUpStarted("api", "dev-ns", "", "wf-1")
		tracker.TrackDeployStarted(DeployStartedMetadata{})
		tracker.TrackWakeTriggered(context.Background(), WakeTriggeredMetadata{})
		tracker.Close()
	})
}

func TestTracker_TrackImageBuild(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{}
	tracker, tracked := newTestTracker(fake, &mockNamespaceUIDResolver{uid: "ns-uid-xyz"})
	meta := &ImageBuildMetadata{
		Name:                     "api",
		Namespace:                "my-ns",
		Success:                  true,
		BuildDuration:            30 * time.Second,
		WaitForBuildkitAvailable: 5 * time.Second,
		ConnectionType:           "proxy",
		RepoURL:                  "https://github.com/org/repo",
	}

	tracker.TrackImageBuild(context.Background(), meta)
	ev := sendOne(t, tracker, fake)

	require.Equal(t, []mockEvent{{event: imageBuildEvent, success: true, props: meta.toMixpanelProps()}}, *tracked)
	require.Equal(t, productImageBuildEvent, ev.event)
	require.Equal(t, "user-123", ev.distinctID)
	require.Equal(t, "api", ev.props["service"])
	require.Equal(t, true, ev.props["result"])
	require.Equal(t, 30, ev.props["duration_seconds"])
	require.Equal(t, 5, ev.props["queue_duration_seconds"])
	require.Equal(t, "proxy", ev.props["connection_type"])
	require.Equal(t, hashedTestRepoURL, ev.props["repo_url"])
	require.Equal(t, "ns-uid-xyz", ev.props["namespace"])
	require.NotContains(t, ev.props, "error_reason")
	require.Equal(t, "ACME Corp", ev.props["customer_name"])
}

func TestTracker_TrackImageBuild_SendsErrorCategory(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{}
	tracker, _ := newTestTracker(fake, nil)

	tracker.TrackImageBuild(context.Background(), &ImageBuildMetadata{Success: false, ErrorReason: "queue_timeout"})
	ev := sendOne(t, tracker, fake)

	require.Equal(t, false, ev.props["result"])
	require.Equal(t, "queue_timeout", ev.props["error_reason"])
}

func TestTracker_TrackImageBuild_OmitsErrorCategoryOnSuccess(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{}
	tracker, _ := newTestTracker(fake, nil)

	tracker.TrackImageBuild(context.Background(), &ImageBuildMetadata{Success: true, ErrorReason: "unknown"})
	ev := sendOne(t, tracker, fake)

	require.NotContains(t, ev.props, "error_reason")
}

func TestTracker_TrackDeployPipelineTriggered(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{}
	tracker, tracked := newTestTracker(fake, &mockNamespaceUIDResolver{uid: "ns-uid-xyz"})

	tracker.TrackDeployPipelineTriggered(context.Background(), DeployPipelineTriggeredMetadata{
		WorkflowID:       "wf-abc-123",
		ParentWorkflowID: "wf-parent-1",
		RepoURL:          "https://github.com/org/repo",
		Namespace:        "my-ns",
		DeployType:       "git_url",
		UIElement:        "deploy-button",
		IsWithinPreview:  true,
	})
	ev := sendOne(t, tracker, fake)

	require.Empty(t, *tracked)
	require.Equal(t, productDeployPipelineTriggeredEvent, ev.event)
	require.Equal(t, "user-123", ev.distinctID)
	require.Equal(t, "wf-abc-123", ev.props["workflow_id"])
	require.Equal(t, "wf-parent-1", ev.props["parent_workflow_id"])
	require.Equal(t, hashedTestRepoURL, ev.props["repo_url"])
	require.Equal(t, "git_url", ev.props["deploy_type"])
	require.Equal(t, true, ev.props["is_within_preview"])
	require.Equal(t, false, ev.props["is_redeploy"])
	require.Equal(t, "ns-uid-xyz", ev.props["namespace"])
	require.NotContains(t, ev.props, "success")
	require.NotContains(t, ev.props, "ui_element", "the CLI never sent ui_element to PostHog")
}

func TestTracker_TrackDeployPipelineTriggered_OmitsEmptyOptionalFields(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{}
	tracker, _ := newTestTracker(fake, nil)

	tracker.TrackDeployPipelineTriggered(context.Background(), DeployPipelineTriggeredMetadata{WorkflowID: "wf-1"})
	ev := sendOne(t, tracker, fake)

	require.NotContains(t, ev.props, "repo_url")
	require.NotContains(t, ev.props, "parent_workflow_id")
}

func TestTracker_TrackDeployPreviewTriggered(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{}
	tracker, _ := newTestTracker(fake, &mockNamespaceUIDResolver{uid: "preview-uid-xyz"})

	tracker.TrackDeployPreviewTriggered(context.Background(), DeployPreviewTriggeredMetadata{
		WorkflowID:       "wf-preview-456",
		ParentWorkflowID: "wf-parent-2",
		RepoURL:          "https://github.com/org/repo",
		Preview:          "my-preview",
		UIElement:        "deploy-button",
		IsRedeploy:       true,
	})
	ev := sendOne(t, tracker, fake)

	require.Equal(t, productDeployPreviewTriggeredEvent, ev.event)
	require.Equal(t, "wf-preview-456", ev.props["workflow_id"])
	require.Equal(t, "wf-parent-2", ev.props["parent_workflow_id"])
	require.Equal(t, hashedTestRepoURL, ev.props["repo_url"])
	require.Equal(t, true, ev.props["is_redeploy"])
	require.Equal(t, "preview-uid-xyz", ev.props["preview"], "the preview UID is sent, never its name")
	require.NotContains(t, ev.props, "namespace")
	require.NotContains(t, ev.props, "ui_element", "the CLI never sent ui_element to PostHog")
}

func TestTracker_TrackDeployPreviewTriggered_OmitsEmptyOptionalFields(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{}
	tracker, _ := newTestTracker(fake, nil)

	tracker.TrackDeployPreviewTriggered(context.Background(), DeployPreviewTriggeredMetadata{WorkflowID: "wf-1"})
	ev := sendOne(t, tracker, fake)

	require.NotContains(t, ev.props, "repo_url")
	require.NotContains(t, ev.props, "parent_workflow_id")
	require.Equal(t, "", ev.props["preview"])
}

func TestTracker_TrackWakeTriggered(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{}
	tracker, _ := newTestTracker(fake, &mockNamespaceUIDResolver{uid: "ns-uid-wake"})

	tracker.TrackWakeTriggered(context.Background(), WakeTriggeredMetadata{Namespace: "dev-ns", IsPreview: true})
	ev := sendOne(t, tracker, fake)

	require.Equal(t, productWakeTriggeredEvent, ev.event)
	require.Equal(t, "ns-uid-wake", ev.props["namespace"])
	require.Equal(t, true, ev.props["is_preview"])
	require.NotContains(t, ev.props, "workflow_id")
}

func TestTracker_TrackUpStarted(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{}
	tracker, tracked := newTestTracker(fake, &mockNamespaceUIDResolver{uid: "ns-uid-456"})

	tracker.TrackUpStarted("api", "dev-ns", "https://github.com/org/repo", "wf-abc-123")
	ev := sendOne(t, tracker, fake)

	require.Empty(t, *tracked)
	require.Equal(t, productUpStartedEvent, ev.event)
	require.Equal(t, "api", ev.props["service"])
	require.Equal(t, "ns-uid-456", ev.props["namespace"])
	require.Equal(t, hashedTestRepoURL, ev.props["repo_url"])
	require.Equal(t, "wf-abc-123", ev.props["workflow_id"])
}

func TestTracker_TrackUpStarted_EmptyFieldsSentAsEmpty(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{}
	tracker, _ := newTestTracker(fake, nil)

	tracker.TrackUpStarted("", "", "", "")
	ev := sendOne(t, tracker, fake)

	require.Equal(t, "", ev.props["service"])
	require.Equal(t, "", ev.props["namespace"])
	require.Equal(t, "", ev.props["repo_url"])
	require.Equal(t, "", ev.props["workflow_id"])
}

func TestTracker_TrackUp(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{}
	tracker, tracked := newTestTracker(fake, &mockNamespaceUIDResolver{uid: "ns-uid-789"})
	meta := &UpMetricsMetadata{
		success:         true,
		manifestType:    model.OktetoManifestType,
		isBuildExecuted: true,
		service:         "api",
		namespace:       "dev-ns",
		repoURL:         "https://github.com/org/repo",
		execDuration:    90 * time.Second,
		isReconnect:     true,
		reconnectCount:  1,
		reconnectCause:  reconnectCauseDefault,
	}

	tracker.TrackUp(meta)
	ev := sendOne(t, tracker, fake)

	require.Equal(t, []mockEvent{{event: upEvent, success: true, props: meta.toProps()}}, *tracked)
	require.Equal(t, productUpEvent, ev.event)
	require.Equal(t, true, ev.props["result"])
	require.Equal(t, "manifest", ev.props["manifest_type"])
	require.Equal(t, true, ev.props["is_build_executed"])
	require.Equal(t, "api", ev.props["service"])
	require.Equal(t, "ns-uid-789", ev.props["namespace"])
	require.Equal(t, hashedTestRepoURL, ev.props["repo_url"])
	require.Equal(t, 90, ev.props["duration_seconds"])
	require.Equal(t, 1, ev.props["reconnect_count"])
	require.Equal(t, "unrecognised", ev.props["reconnect_cause"])
	require.NotContains(t, ev.props, "error_reason")
}

func TestTracker_TrackUp_FailureIncludesErrorReason(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{}
	tracker, _ := newTestTracker(fake, nil)

	tracker.TrackUp(&UpMetricsMetadata{success: false, failActivate: true})
	ev := sendOne(t, tracker, fake)

	require.Equal(t, false, ev.props["result"])
	require.Equal(t, "fail_activate", ev.props["error_reason"])
}

func TestTracker_TrackDeployStarted(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{}
	tracker, tracked := newTestTracker(fake, &mockNamespaceUIDResolver{uid: "ns-uid-654"})

	tracker.TrackDeployStarted(DeployStartedMetadata{
		Namespace:  "dev-ns",
		RepoURL:    "https://github.com/org/repo",
		WorkflowID: "wf-deploy-1",
		IsPreview:  true,
		IsRedeploy: true,
	})
	ev := sendOne(t, tracker, fake)

	require.Empty(t, *tracked)
	require.Equal(t, productDeployStartedEvent, ev.event)
	require.Equal(t, "ns-uid-654", ev.props["namespace"])
	require.Equal(t, hashedTestRepoURL, ev.props["repo_url"])
	require.Equal(t, "wf-deploy-1", ev.props["workflow_id"])
	require.Equal(t, true, ev.props["is_within_preview"])
	require.Equal(t, true, ev.props["is_redeploy"])
	require.NotContains(t, ev.props, "success", "deploy_started never had a success property")
	require.NotContains(t, ev.props, "result")
}

func TestTracker_TrackDeploy(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{}
	tracker, tracked := newTestTracker(fake, &mockNamespaceUIDResolver{uid: "ns-uid-789"})

	tracker.TrackDeploy(DeployMetadata{
		Success:                true,
		PipelineType:           model.OktetoManifestType,
		Namespace:              "dev-ns",
		WorkflowID:             "wf-deploy-1",
		Duration:               45 * time.Second,
		IsRedeploy:             true,
		HasDependenciesSection: true,
		WaitForDependencies:    true,
	})
	ev := sendOne(t, tracker, fake)

	require.Len(t, *tracked, 1)
	require.Equal(t, deployEvent, (*tracked)[0].event)
	require.Equal(t, productDeployCompletedEvent, ev.event)
	require.Equal(t, true, ev.props["result"])
	require.Equal(t, "manifest", ev.props["manifest_archetype"])
	require.Equal(t, "wf-deploy-1", ev.props["workflow_id"])
	require.Equal(t, "ns-uid-789", ev.props["namespace"])
	require.Equal(t, float64(45), ev.props["duration_seconds"])
	require.Equal(t, "regular", ev.props["namespace_type"])
	require.Equal(t, true, ev.props["is_redeploy"])
	require.Equal(t, true, ev.props["has_dependencies_section"])
	require.Equal(t, true, ev.props["wait_for_dependencies"])
	require.NotContains(t, ev.props, "error_reason")
}

func TestTracker_TrackDeploy_UnknownErrorOmitsErrorReason(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{}
	tracker, _ := newTestTracker(fake, nil)

	tracker.TrackDeploy(DeployMetadata{Success: false, Err: assert.AnError})
	ev := sendOne(t, tracker, fake)

	require.NotContains(t, ev.props, "error_reason")
}

func TestTracker_TrackDeploy_KnownErrorsSetErrorReason(t *testing.T) {
	tests := []struct {
		err            error
		name           string
		expectedReason string
	}{
		{name: "no deploy commands", err: oktetoErrors.ErrManifestFoundButNoDeployAndDependenciesCommands, expectedReason: "no_deploy_commands"},
		{name: "timeout", err: oktetoErrors.ErrTimeout, expectedReason: "timeout"},
		{name: "command failed", err: oktetoErrors.ErrCommandFailed, expectedReason: "command_failed"},
		{name: "internal server error", err: oktetoErrors.ErrInternalServerError, expectedReason: "internal_server_error"},
		{name: "user error", err: oktetoErrors.UserError{E: errors.New("something went wrong")}, expectedReason: "user_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupAnalyticsContext(t, true)
			fake := &fakeEventTracker{}
			tracker, _ := newTestTracker(fake, nil)

			tracker.TrackDeploy(DeployMetadata{Success: false, Err: tt.err})
			ev := sendOne(t, tracker, fake)

			require.Equal(t, tt.expectedReason, ev.props["error_reason"])
		})
	}
}
