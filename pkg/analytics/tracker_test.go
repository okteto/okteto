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
	tracker, tracked := newTestTracker(fake, &mockNamespaceUIDResolver{uid: testNamespaceUID})
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

	require.Empty(t, *tracked, "imageBuild is sent with the product properties, not through trackFn")
	require.Equal(t, imageBuildEvent, ev.event)
	require.Equal(t, "user-123", ev.distinctID)
	requireContainsProps(t, meta.toProps(), ev.props)
	require.Equal(t, true, ev.props["success"])
	require.Equal(t, testNamespaceUID, ev.props["namespace"])
	require.Equal(t, "ACME Corp", ev.props["customerName"])
}

func TestTracker_TrackDeployPipelineTriggered(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{}
	tracker, tracked := newTestTracker(fake, &mockNamespaceUIDResolver{uid: testNamespaceUID})

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
	require.Equal(t, "wf-abc-123", ev.props["workflowID"])
	require.Equal(t, "wf-parent-1", ev.props["parentWorkflowID"])
	require.Equal(t, hashedTestRepoURL, ev.props["repoURL"])
	require.Equal(t, "git_url", ev.props["deploySource"])
	require.Equal(t, true, ev.props["isPreview"])
	require.Equal(t, false, ev.props["isRedeploy"])
	require.Equal(t, testNamespaceUID, ev.props["namespace"])
	require.NotContains(t, ev.props, "deployType", "Deploy.deployType holds other values")
	require.NotContains(t, ev.props, "success")
	require.NotContains(t, ev.props, "uiElement", "the CLI never sent ui_element to PostHog")
}

func TestTracker_TrackDeployPipelineTriggered_OmitsEmptyOptionalFields(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{}
	tracker, _ := newTestTracker(fake, nil)

	tracker.TrackDeployPipelineTriggered(context.Background(), DeployPipelineTriggeredMetadata{WorkflowID: "wf-1"})
	ev := sendOne(t, tracker, fake)

	require.NotContains(t, ev.props, "repoURL")
	require.NotContains(t, ev.props, "parentWorkflowID")
}

func TestTracker_TrackDeployPreviewTriggered(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{}
	tracker, _ := newTestTracker(fake, &mockNamespaceUIDResolver{uid: testPreviewUID})

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
	require.Equal(t, "wf-preview-456", ev.props["workflowID"])
	require.Equal(t, "wf-parent-2", ev.props["parentWorkflowID"])
	require.Equal(t, hashedTestRepoURL, ev.props["repoURL"])
	require.Equal(t, true, ev.props["isRedeploy"])
	require.Equal(t, testPreviewUID, ev.props["preview"], "the preview UID is sent, never its name")
	require.NotContains(t, ev.props, "namespace")
	require.NotContains(t, ev.props, "uiElement", "the CLI never sent ui_element to PostHog")
}

func TestTracker_TrackDeployPreviewTriggered_OmitsEmptyOptionalFields(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{}
	tracker, _ := newTestTracker(fake, nil)

	tracker.TrackDeployPreviewTriggered(context.Background(), DeployPreviewTriggeredMetadata{WorkflowID: "wf-1"})
	ev := sendOne(t, tracker, fake)

	require.NotContains(t, ev.props, "repoURL")
	require.NotContains(t, ev.props, "parentWorkflowID")
	require.Equal(t, "", ev.props["preview"])
}

func TestTracker_TrackWakeTriggered(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{}
	tracker, _ := newTestTracker(fake, &mockNamespaceUIDResolver{uid: testNamespaceUID})

	tracker.TrackWakeTriggered(context.Background(), WakeTriggeredMetadata{Namespace: "dev-ns", IsPreview: true})
	ev := sendOne(t, tracker, fake)

	require.Equal(t, productWakeTriggeredEvent, ev.event)
	require.Equal(t, testNamespaceUID, ev.props["namespace"])
	require.Equal(t, true, ev.props["isPreview"])
	require.NotContains(t, ev.props, "workflowID")
}

func TestTracker_TrackUpStarted(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{}
	tracker, tracked := newTestTracker(fake, &mockNamespaceUIDResolver{uid: testNamespaceUID})

	tracker.TrackUpStarted("api", "dev-ns", "https://github.com/org/repo", "wf-abc-123")
	ev := sendOne(t, tracker, fake)

	require.Empty(t, *tracked)
	require.Equal(t, productUpStartedEvent, ev.event)
	require.Equal(t, hashString("api"), ev.props["service"], "service names are never sent raw")
	require.Equal(t, testNamespaceUID, ev.props["namespace"])
	require.Equal(t, hashedTestRepoURL, ev.props["repoURL"])
	require.Equal(t, "wf-abc-123", ev.props["workflowID"])
}

func TestTracker_TrackUpStarted_EmptyFieldsSentAsEmpty(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{}
	tracker, _ := newTestTracker(fake, nil)

	tracker.TrackUpStarted("", "", "", "")
	ev := sendOne(t, tracker, fake)

	require.Equal(t, "", ev.props["service"])
	require.Equal(t, "", ev.props["namespace"])
	require.Equal(t, "", ev.props["repoURL"])
	require.Equal(t, "", ev.props["workflowID"])
}

func TestTracker_TrackUp(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{}
	tracker, tracked := newTestTracker(fake, &mockNamespaceUIDResolver{uid: testNamespaceUID})
	meta := &UpMetricsMetadata{
		workflowID:      "wf-up-1",
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

	require.Empty(t, *tracked, "Up is sent with the product properties, not through trackFn")
	require.Equal(t, upEvent, ev.event)
	requireContainsProps(t, meta.toProps(), ev.props)
	requireContainsProps(t, meta.productProps(), ev.props)
	require.Equal(t, true, ev.props["success"])
	require.Equal(t, "wf-up-1", ev.props["workflowID"])
	require.Equal(t, testNamespaceUID, ev.props["namespace"])
}

func TestTracker_TrackDeployStarted(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{}
	tracker, tracked := newTestTracker(fake, &mockNamespaceUIDResolver{uid: testNamespaceUID})

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
	require.Equal(t, testNamespaceUID, ev.props["namespace"])
	require.Equal(t, hashedTestRepoURL, ev.props["repoURL"])
	require.Equal(t, "wf-deploy-1", ev.props["workflowID"])
	require.Equal(t, true, ev.props["isPreview"])
	require.Equal(t, true, ev.props["isRedeploy"])
	require.NotContains(t, ev.props, "success", "deploy_started never had a success property")
	require.NotContains(t, ev.props, "result")
}

func TestTracker_TrackDeploy(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{}
	tracker, tracked := newTestTracker(fake, &mockNamespaceUIDResolver{uid: testNamespaceUID})
	meta := DeployMetadata{
		Success:                true,
		PipelineType:           model.OktetoManifestType,
		Namespace:              "dev-ns",
		WorkflowID:             "wf-deploy-1",
		Duration:               45 * time.Second,
		IsRedeploy:             true,
		HasDependenciesSection: true,
		WaitForDependencies:    true,
	}

	tracker.TrackDeploy(meta)
	ev := sendOne(t, tracker, fake)

	require.Empty(t, *tracked, "Deploy is sent with the product properties, not through trackFn")
	require.Equal(t, deployEvent, ev.event)
	requireContainsProps(t, meta.toProps(), ev.props)
	require.Equal(t, true, ev.props["success"])
	require.Equal(t, "wf-deploy-1", ev.props["workflowID"])
	require.Equal(t, testNamespaceUID, ev.props["namespace"])
	require.NotContains(t, ev.props, "error")
}

func TestTracker_TrackDeploy_ErrorIsCategorized(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{}
	tracker, _ := newTestTracker(fake, nil)

	tracker.TrackDeploy(DeployMetadata{Success: false, Err: oktetoErrors.UserError{E: errors.New("error executing command './deploy.sh --password=pass': exit status 1")}})
	ev := sendOne(t, tracker, fake)

	require.Equal(t, false, ev.props["success"])
	require.Equal(t, "user_error", ev.props["error"])
}

// requireContainsProps checks that every property in expected was sent with the same value.
func requireContainsProps(t *testing.T, expected, sent map[string]any) {
	t.Helper()
	for k, v := range expected {
		require.Contains(t, sent, k)
		require.Equal(t, v, sent[k], k)
	}
}
