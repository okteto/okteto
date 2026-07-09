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
	"sync"
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/posthog/posthog-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPostHogClient captures Enqueue calls for assertion.
type mockPostHogClient struct {
	captured []posthog.Capture
	err      error
	done     chan struct{}
	once     sync.Once
}

func (m *mockPostHogClient) Enqueue(msg posthog.Message) error {
	if c, ok := msg.(posthog.Capture); ok {
		m.captured = append(m.captured, c)
	}
	if m.done != nil {
		m.once.Do(func() { close(m.done) })
	}
	return m.err
}

func (m *mockPostHogClient) Close() error { return nil }

// waitCapture blocks until the first Enqueue call or the test times out.
func (m *mockPostHogClient) waitCapture(t *testing.T) {
	t.Helper()
	select {
	case <-m.done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for PostHog Enqueue")
	}
}

func setupPostHogContext(t *testing.T, analyticsEnabled bool) func() {
	t.Helper()
	prevAnalytics := currentAnalytics
	prevStore := okteto.CurrentStore
	prevVersion := config.VersionString

	currentAnalytics = &Analytics{Enabled: analyticsEnabled, MachineID: "test-machine"}
	config.VersionString = "test-version"
	okteto.CurrentStore = &okteto.ContextStore{
		CurrentContext: "https://cloud.okteto.net",
		Contexts: map[string]*okteto.Context{
			"https://cloud.okteto.net": {
				Name:           "https://cloud.okteto.net",
				UserID:         "user-123",
				CompanyName:    "ACME Corp",
				ClusterID:      "cluster-uuid-1234",
				ClusterVersion: "1.2.3",
				Analytics:      true,
			},
		},
	}
	return func() {
		currentAnalytics = prevAnalytics
		okteto.CurrentStore = prevStore
		config.VersionString = prevVersion
	}
}

func TestPostHogBackend_TrackImageBuild_NilClient(t *testing.T) {
	b := &posthogBackend{client: nil}
	require.NotPanics(t, func() {
		b.TrackImageBuild(context.Background(), &ImageBuildMetadata{Success: true})
	})
}

func TestPostHogBackend_TrackImageBuild_AnalyticsDisabled(t *testing.T) {
	teardown := setupPostHogContext(t, false /* disabled */)
	defer teardown()

	mock := &mockPostHogClient{}
	b := &posthogBackend{client: mock}
	b.TrackImageBuild(context.Background(), &ImageBuildMetadata{Success: true})

	require.Empty(t, mock.captured, "Enqueue must not be called when analytics is disabled")
}

func TestPostHogBackend_TrackImageBuild_ContextNotInitialized(t *testing.T) {
	// Cannot use setupPostHogContext here because it sets CurrentContext to a
	// non-empty string. We need an empty CurrentContext to test the guard.
	prevAnalytics := currentAnalytics
	prevStore := okteto.CurrentStore
	currentAnalytics = &Analytics{Enabled: true}
	okteto.CurrentStore = &okteto.ContextStore{CurrentContext: ""}
	defer func() {
		currentAnalytics = prevAnalytics
		okteto.CurrentStore = prevStore
	}()

	mock := &mockPostHogClient{}
	b := &posthogBackend{client: mock}
	b.TrackImageBuild(context.Background(), &ImageBuildMetadata{Success: true})

	require.Empty(t, mock.captured, "Enqueue must not be called when context is not initialized")
}

func TestPostHogBackend_TrackImageBuild_HappyPath(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()
	t.Setenv("OKTETO_ORIGIN", "")
	t.Setenv("OKTETO_WITHIN_DEPLOY_COMMAND_CONTEXT", "")

	mock := &mockPostHogClient{done: make(chan struct{})}
	b := &posthogBackend{client: mock}

	meta := &ImageBuildMetadata{
		Name:                     "api",
		Success:                  true,
		BuildDuration:            30 * time.Second,
		WaitForBuildkitAvailable: 5 * time.Second,
		BuildkitDuration:         25 * time.Second,
		ContextTransferDuration:  3 * time.Second,
		BuildContextSize:         20_000_000,
		CacheHit:                 false,
		ConnectionType:           "proxy",
		RepoURL:                  "https://github.com/org/repo",
	}
	b.TrackImageBuild(context.Background(), meta)
	mock.waitCapture(t)

	require.Len(t, mock.captured, 1)
	event := mock.captured[0]
	require.Equal(t, posthogImageBuildEvent, event.Event)
	require.Equal(t, "user-123", event.DistinctId)
	require.Equal(t, "api", event.Properties["service"])
	require.Equal(t, 30, event.Properties["duration_seconds"])
	require.Equal(t, 5, event.Properties["queue_duration_seconds"])
	require.Equal(t, int64(3000), event.Properties["build_context_duration_ms"])
	require.Equal(t, true, event.Properties["result"])
	require.Equal(t, int64(20_000_000), event.Properties["build_context_size_bytes"])
	require.Equal(t, false, event.Properties["is_cache"])
	require.Equal(t, "proxy", event.Properties["connection_type"])
	require.Equal(t, "bdb72e6e68b80f9ed3bbdb0ad1d2f8b4fac8ade379eb82182de40a3357a2d3b3", event.Properties["repo_url"])
	require.NotContains(t, event.Properties, "error_reason")
	require.NotEmpty(t, event.Properties["cli_version"])
	require.NotEmpty(t, event.Properties["os"])
	require.NotEmpty(t, event.Properties["arch"])
	require.Equal(t, "test-machine", event.Properties["machine_id"])

	// Common props
	require.Equal(t, "ACME Corp", event.Properties["customer_name"])
	require.Equal(t, "cluster-uuid-1234", event.Properties["cluster_id"])
	require.Equal(t, "https://cloud.okteto.net", event.Properties["cluster_url"])
	require.Equal(t, "1.2.3", event.Properties["cluster_version"])
	require.Equal(t, "user-123", event.Properties["user_id"])
	require.Equal(t, "cli", event.Properties["trigger_source"])
}

func TestPostHogBackend_TrackImageBuild_EnqueueError(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	mock := &mockPostHogClient{err: errors.New("posthog unavailable"), done: make(chan struct{})}
	b := &posthogBackend{client: mock}

	b.TrackImageBuild(context.Background(), &ImageBuildMetadata{Success: true})
	mock.waitCapture(t)
}

func TestPostHogBackend_AgentType_OmittedWhenNoAgent(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	t.Setenv("CODEX_CI", "")
	t.Setenv("GEMINI_CLI", "")
	t.Setenv("CLAUDECODE", "")
	t.Setenv("CURSOR_SANDBOX", "")

	mock := &mockPostHogClient{done: make(chan struct{})}
	b := &posthogBackend{client: mock}
	b.TrackImageBuild(context.Background(), &ImageBuildMetadata{})
	mock.waitCapture(t)

	require.Len(t, mock.captured, 1)
	require.NotContains(t, mock.captured[0].Properties, "agent_type")
	require.Equal(t, false, mock.captured[0].Properties["is_agent"])
}

func TestPostHogBackend_AgentType_PresentWhenAgent(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	t.Setenv("CODEX_CI", "")
	t.Setenv("GEMINI_CLI", "")
	t.Setenv("CLAUDECODE", "true")
	t.Setenv("CURSOR_SANDBOX", "")

	mock := &mockPostHogClient{done: make(chan struct{})}
	b := &posthogBackend{client: mock}
	b.TrackImageBuild(context.Background(), &ImageBuildMetadata{Success: true})
	mock.waitCapture(t)

	require.Len(t, mock.captured, 1)
	require.Equal(t, true, mock.captured[0].Properties["is_agent"])
	require.Equal(t, "claude_code", mock.captured[0].Properties["agent_type"])
}

// mockNamespaceUIDResolver is a test double for NamespaceUIDResolver.
type mockNamespaceUIDResolver struct {
	uid string
	err error
}

func (m *mockNamespaceUIDResolver) GetNamespaceUID(_ context.Context, _ string) (string, error) {
	return m.uid, m.err
}

func TestTrackImageBuild_IncludesNamespace(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	mock := &mockPostHogClient{done: make(chan struct{})}
	b := &posthogBackend{
		client:     mock,
		nsResolver: &mockNamespaceUIDResolver{uid: "ns-uid-xyz"},
	}

	b.TrackImageBuild(context.Background(), &ImageBuildMetadata{
		Namespace: "my-ns",
		Success:   true,
	})
	mock.waitCapture(t)

	require.Len(t, mock.captured, 1)
	require.Equal(t, "ns-uid-xyz", mock.captured[0].Properties["namespace"])
}

func TestTrackImageBuild_NilResolver(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	mock := &mockPostHogClient{done: make(chan struct{})}
	b := &posthogBackend{client: mock, nsResolver: nil}

	b.TrackImageBuild(context.Background(), &ImageBuildMetadata{
		Namespace: "my-ns",
		Success:   true,
	})
	mock.waitCapture(t)

	require.Len(t, mock.captured, 1)
	require.Equal(t, "", mock.captured[0].Properties["namespace"])
}

func TestTrackImageBuild_ResolverError(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	mock := &mockPostHogClient{done: make(chan struct{})}
	b := &posthogBackend{
		client:     mock,
		nsResolver: &mockNamespaceUIDResolver{err: errors.New("k8s unavailable")},
	}

	b.TrackImageBuild(context.Background(), &ImageBuildMetadata{
		Namespace: "my-ns",
		Success:   true,
	})
	mock.waitCapture(t)

	// Event must be sent with empty namespace when the resolver fails
	require.Len(t, mock.captured, 1)
	require.Equal(t, "", mock.captured[0].Properties["namespace"])
}

func TestPostHogBackend_enqueue_appliesEnrichersBeforeSending(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	mock := &mockPostHogClient{done: make(chan struct{})}
	b := &posthogBackend{client: mock}

	enricher := func(_ context.Context, props posthog.Properties) {
		props["enriched_key"] = "enriched_value"
	}

	props := posthog.Properties{}
	b.enqueue(context.Background(), "user-123", "test_event", props, enricher)
	mock.waitCapture(t)

	require.Len(t, mock.captured, 1)
	require.Equal(t, "test_event", mock.captured[0].Event)
	require.Equal(t, "user-123", mock.captured[0].DistinctId)
	require.Equal(t, "enriched_value", mock.captured[0].Properties["enriched_key"])
}

func TestPostHogBackend_enqueue_sendsEventWithNoEnrichers(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	mock := &mockPostHogClient{done: make(chan struct{})}
	b := &posthogBackend{client: mock}

	props := posthog.Properties{"key": "val"}
	b.enqueue(context.Background(), "user-abc", "bare_event", props)
	mock.waitCapture(t)

	require.Len(t, mock.captured, 1)
	require.Equal(t, "bare_event", mock.captured[0].Event)
	require.Equal(t, "val", mock.captured[0].Properties["key"])
}

func TestPostHogBackend_withNamespace_addsUID(t *testing.T) {
	b := &posthogBackend{nsResolver: &mockNamespaceUIDResolver{uid: "ns-uid-abc"}}

	props := posthog.Properties{}
	b.withNamespace("my-ns")(context.Background(), props)

	require.Equal(t, "ns-uid-abc", props["namespace"])
}

func TestPostHogBackend_withNamespace_setsEmptyWhenNamespaceEmpty(t *testing.T) {
	b := &posthogBackend{nsResolver: &mockNamespaceUIDResolver{uid: "ns-uid-abc"}}

	props := posthog.Properties{}
	b.withNamespace("")(context.Background(), props)

	require.Equal(t, "", props["namespace"])
}

func TestPostHogBackend_withNamespace_setsEmptyWhenResolverNil(t *testing.T) {
	b := &posthogBackend{nsResolver: nil}

	props := posthog.Properties{}
	b.withNamespace("my-ns")(context.Background(), props)

	require.Equal(t, "", props["namespace"])
}

func TestPostHogBackend_withNamespace_setsEmptyOnResolverError(t *testing.T) {
	b := &posthogBackend{nsResolver: &mockNamespaceUIDResolver{err: errors.New("k8s down")}}

	props := posthog.Properties{}
	b.withNamespace("my-ns")(context.Background(), props)

	require.Equal(t, "", props["namespace"])
}

func TestPostHogBackend_withPreview_addsUID(t *testing.T) {
	b := &posthogBackend{nsResolver: &mockNamespaceUIDResolver{uid: "preview-uid-abc"}}

	props := posthog.Properties{}
	b.withPreview("my-preview")(context.Background(), props)

	require.Equal(t, "preview-uid-abc", props["preview"])
}

func TestPostHogBackend_withPreview_setsEmptyWhenPreviewEmpty(t *testing.T) {
	b := &posthogBackend{nsResolver: &mockNamespaceUIDResolver{uid: "preview-uid-abc"}}

	props := posthog.Properties{}
	b.withPreview("")(context.Background(), props)

	require.Equal(t, "", props["preview"])
}

func TestPostHogBackend_withPreview_setsEmptyWhenResolverNil(t *testing.T) {
	b := &posthogBackend{nsResolver: nil}

	props := posthog.Properties{}
	b.withPreview("my-preview")(context.Background(), props)

	require.Equal(t, "", props["preview"])
}

func TestPostHogBackend_withPreview_setsEmptyOnResolverError(t *testing.T) {
	b := &posthogBackend{nsResolver: &mockNamespaceUIDResolver{err: errors.New("k8s down")}}

	props := posthog.Properties{}
	b.withPreview("my-preview")(context.Background(), props)

	require.Equal(t, "", props["preview"])
}

func TestIsWithinPreview_TrueWhenEnvSet(t *testing.T) {
	t.Setenv("OKTETO_IS_PREVIEW_ENVIRONMENT", "true")
	require.True(t, IsWithinPreview(context.Background(), nil))
}

func TestIsWithinPreview_FalseWhenEnvEmpty(t *testing.T) {
	t.Setenv("OKTETO_IS_PREVIEW_ENVIRONMENT", "")
	require.False(t, IsWithinPreview(context.Background(), nil))
}

func TestIsWithinPreview_FalseWhenEnvOtherValue(t *testing.T) {
	t.Setenv("OKTETO_IS_PREVIEW_ENVIRONMENT", "false")
	require.False(t, IsWithinPreview(context.Background(), nil))
}

func TestIsWithinPreview_TrueWhenPreviewGetSucceeds(t *testing.T) {
	t.Setenv("OKTETO_IS_PREVIEW_ENVIRONMENT", "")
	prevStore := okteto.CurrentStore
	okteto.CurrentStore = &okteto.ContextStore{
		CurrentContext: "https://cloud.okteto.net",
		Contexts: map[string]*okteto.Context{
			"https://cloud.okteto.net": {Namespace: "my-preview-ns"},
		},
	}
	defer func() { okteto.CurrentStore = prevStore }()

	checker := func(_ context.Context, _ string) error { return nil }
	require.True(t, IsWithinPreview(context.Background(), checker))
}

func TestIsWithinPreview_FalseWhenPreviewGetFails(t *testing.T) {
	t.Setenv("OKTETO_IS_PREVIEW_ENVIRONMENT", "")
	prevStore := okteto.CurrentStore
	okteto.CurrentStore = &okteto.ContextStore{
		CurrentContext: "https://cloud.okteto.net",
		Contexts: map[string]*okteto.Context{
			"https://cloud.okteto.net": {Namespace: "regular-ns"},
		},
	}
	defer func() { okteto.CurrentStore = prevStore }()

	checker := func(_ context.Context, _ string) error { return errors.New("not found") }
	require.False(t, IsWithinPreview(context.Background(), checker))
}

func TestIsWithinPreview_FalseWhenNamespaceEmpty(t *testing.T) {
	t.Setenv("OKTETO_IS_PREVIEW_ENVIRONMENT", "")
	prevStore := okteto.CurrentStore
	okteto.CurrentStore = &okteto.ContextStore{
		CurrentContext: "https://cloud.okteto.net",
		Contexts: map[string]*okteto.Context{
			"https://cloud.okteto.net": {Namespace: ""},
		},
	}
	defer func() { okteto.CurrentStore = prevStore }()

	called := false
	checker := func(_ context.Context, _ string) error { called = true; return nil }
	require.False(t, IsWithinPreview(context.Background(), checker))
	require.False(t, called, "checker must not be called when namespace is empty")
}

func TestPostHogBackend_TrackDeployPipelineTriggered_NilClient(t *testing.T) {
	b := &posthogBackend{client: nil}
	require.NotPanics(t, func() {
		b.TrackDeployPipelineTriggered(context.Background(), DeployPipelineTriggeredMetadata{WorkflowID: "wf-1"})
	})
}

func TestPostHogBackend_TrackDeployPipelineTriggered_AnalyticsDisabled(t *testing.T) {
	teardown := setupPostHogContext(t, false)
	defer teardown()

	mock := &mockPostHogClient{}
	b := &posthogBackend{client: mock}
	b.TrackDeployPipelineTriggered(context.Background(), DeployPipelineTriggeredMetadata{WorkflowID: "wf-1"})

	require.Empty(t, mock.captured, "Enqueue must not be called when analytics is disabled")
}

func TestPostHogBackend_TrackDeployPipelineTriggered_HappyPath(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	mock := &mockPostHogClient{done: make(chan struct{})}
	b := &posthogBackend{
		client:     mock,
		nsResolver: &mockNamespaceUIDResolver{uid: "ns-uid-xyz"},
	}

	m := DeployPipelineTriggeredMetadata{
		WorkflowID:       "wf-abc-123",
		ParentWorkflowID: "wf-parent-1",
		RepoURL:          "https://github.com/org/repo",
		Namespace:        "my-ns",
		DeployType:       "git_url",
		IsWithinPreview:  true,
		IsRedeploy:       false,
	}
	b.TrackDeployPipelineTriggered(context.Background(), m)
	mock.waitCapture(t)

	require.Len(t, mock.captured, 1)
	event := mock.captured[0]
	require.Equal(t, posthogDeployPipelineTriggeredEvent, event.Event)
	require.Equal(t, "user-123", event.DistinctId)
	require.Equal(t, "wf-abc-123", event.Properties["workflow_id"])
	require.Equal(t, "wf-parent-1", event.Properties["parent_workflow_id"])
	require.Equal(t, "bdb72e6e68b80f9ed3bbdb0ad1d2f8b4fac8ade379eb82182de40a3357a2d3b3", event.Properties["repo_url"])
	require.Equal(t, "git_url", event.Properties["deploy_type"])
	require.Equal(t, true, event.Properties["is_within_preview"])
	require.Equal(t, false, event.Properties["is_redeploy"])
	require.Equal(t, "ns-uid-xyz", event.Properties["namespace"])
	require.NotEmpty(t, event.Properties["cli_version"])
}

func TestPostHogBackend_TrackDeployPipelineTriggered_OmitsRepoURLWhenEmpty(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	mock := &mockPostHogClient{done: make(chan struct{})}
	b := &posthogBackend{client: mock}
	b.TrackDeployPipelineTriggered(context.Background(), DeployPipelineTriggeredMetadata{WorkflowID: "wf-1", RepoURL: ""})
	mock.waitCapture(t)

	require.Len(t, mock.captured, 1)
	require.NotContains(t, mock.captured[0].Properties, "repo_url")
	require.NotContains(t, mock.captured[0].Properties, "parent_workflow_id")
}

func TestPostHogBackend_TriggerSourceFromOrigin(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	t.Setenv("OKTETO_ORIGIN", "web")
	mock := &mockPostHogClient{done: make(chan struct{})}
	b := &posthogBackend{client: mock}
	b.TrackDeployPipelineTriggered(context.Background(), DeployPipelineTriggeredMetadata{WorkflowID: "wf-1"})
	mock.waitCapture(t)

	require.Len(t, mock.captured, 1)
	require.Equal(t, "web", mock.captured[0].Properties["trigger_source"])
}

func TestPostHogBackend_TriggerSourceDefaultsToCLI(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	t.Setenv("OKTETO_ORIGIN", "")
	t.Setenv("OKTETO_WITHIN_DEPLOY_COMMAND_CONTEXT", "")
	mock := &mockPostHogClient{done: make(chan struct{})}
	b := &posthogBackend{client: mock}
	b.TrackDeployPipelineTriggered(context.Background(), DeployPipelineTriggeredMetadata{WorkflowID: "wf-1"})
	mock.waitCapture(t)

	require.Len(t, mock.captured, 1)
	require.Equal(t, "cli", mock.captured[0].Properties["trigger_source"])
}

func TestPostHogBackend_TriggerSourceWithinDeployContext(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	t.Setenv("OKTETO_ORIGIN", "")
	t.Setenv("OKTETO_WITHIN_DEPLOY_COMMAND_CONTEXT", "true")
	mock := &mockPostHogClient{done: make(chan struct{})}
	b := &posthogBackend{client: mock}
	b.TrackDeployPipelineTriggered(context.Background(), DeployPipelineTriggeredMetadata{WorkflowID: "wf-1"})
	mock.waitCapture(t)

	require.Len(t, mock.captured, 1)
	require.Equal(t, "okteto-deploy", mock.captured[0].Properties["trigger_source"])
}

// is_automation is true when the actor is a CI run or an AI agent, and false for a manual human run.

func TestPostHogBackend_IsAutomation_FalseForManualRun(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	t.Setenv("CI", "")
	t.Setenv("CLAUDECODE", "")
	t.Setenv("CODEX_CI", "")
	t.Setenv("GEMINI_CLI", "")
	t.Setenv("CURSOR_SANDBOX", "")
	mock := &mockPostHogClient{done: make(chan struct{})}
	b := &posthogBackend{client: mock}
	b.TrackDeployPipelineTriggered(context.Background(), DeployPipelineTriggeredMetadata{WorkflowID: "wf-1"})
	mock.waitCapture(t)

	require.Len(t, mock.captured, 1)
	props := mock.captured[0].Properties
	require.Equal(t, false, props["is_ci"])
	require.Equal(t, false, props["is_agent"])
	require.Equal(t, false, props["is_automation"])
}

func TestPostHogBackend_IsCI_EnablesAutomation(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	t.Setenv("CI", "true")
	t.Setenv("CLAUDECODE", "")
	mock := &mockPostHogClient{done: make(chan struct{})}
	b := &posthogBackend{client: mock}
	b.TrackDeployPipelineTriggered(context.Background(), DeployPipelineTriggeredMetadata{WorkflowID: "wf-1"})
	mock.waitCapture(t)

	require.Len(t, mock.captured, 1)
	props := mock.captured[0].Properties
	require.Equal(t, true, props["is_ci"])
	require.Equal(t, true, props["is_automation"])
}

func TestPostHogBackend_IsAgent_EnablesAutomation(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	t.Setenv("CI", "")
	t.Setenv("CLAUDECODE", "true")
	mock := &mockPostHogClient{done: make(chan struct{})}
	b := &posthogBackend{client: mock}
	b.TrackDeployPipelineTriggered(context.Background(), DeployPipelineTriggeredMetadata{WorkflowID: "wf-1"})
	mock.waitCapture(t)

	require.Len(t, mock.captured, 1)
	props := mock.captured[0].Properties
	require.Equal(t, true, props["is_agent"])
	require.Equal(t, false, props["is_ci"])
	require.Equal(t, true, props["is_automation"])
}

func TestPostHogBackend_TrackDeployPreviewTriggered_NilClient(t *testing.T) {
	b := &posthogBackend{client: nil}
	require.NotPanics(t, func() {
		b.TrackDeployPreviewTriggered(context.Background(), DeployPreviewTriggeredMetadata{WorkflowID: "wf-1"})
	})
}

func TestPostHogBackend_TrackDeployPreviewTriggered_AnalyticsDisabled(t *testing.T) {
	teardown := setupPostHogContext(t, false)
	defer teardown()

	mock := &mockPostHogClient{}
	b := &posthogBackend{client: mock}
	b.TrackDeployPreviewTriggered(context.Background(), DeployPreviewTriggeredMetadata{WorkflowID: "wf-1"})

	require.Empty(t, mock.captured, "Enqueue must not be called when analytics is disabled")
}

func TestPostHogBackend_TrackDeployPreviewTriggered_HappyPath(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	mock := &mockPostHogClient{done: make(chan struct{})}
	b := &posthogBackend{
		client:     mock,
		nsResolver: &mockNamespaceUIDResolver{uid: "preview-uid-xyz"},
	}

	m := DeployPreviewTriggeredMetadata{
		WorkflowID:       "wf-preview-456",
		ParentWorkflowID: "wf-parent-2",
		RepoURL:          "https://github.com/org/repo",
		Preview:          "my-preview",
		IsRedeploy:       true,
	}
	b.TrackDeployPreviewTriggered(context.Background(), m)
	mock.waitCapture(t)

	require.Len(t, mock.captured, 1)
	event := mock.captured[0]
	require.Equal(t, posthogDeployPreviewTriggeredEvent, event.Event)
	require.Equal(t, "user-123", event.DistinctId)
	require.Equal(t, "wf-parent-2", event.Properties["parent_workflow_id"])
	require.Equal(t, "bdb72e6e68b80f9ed3bbdb0ad1d2f8b4fac8ade379eb82182de40a3357a2d3b3", event.Properties["repo_url"])
	require.Equal(t, true, event.Properties["is_redeploy"])
	require.Equal(t, "preview-uid-xyz", event.Properties["preview"])
	require.NotEmpty(t, event.Properties["cli_version"])
}

func TestPostHogBackend_TrackDeployPreviewTriggered_OmitsRepoURLWhenEmpty(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	mock := &mockPostHogClient{done: make(chan struct{})}
	b := &posthogBackend{client: mock}
	b.TrackDeployPreviewTriggered(context.Background(), DeployPreviewTriggeredMetadata{WorkflowID: "wf-1", RepoURL: ""})
	mock.waitCapture(t)

	require.Len(t, mock.captured, 1)
	require.NotContains(t, mock.captured[0].Properties, "repo_url")
}

func TestPostHogBackend_TrackUp_NilClient(t *testing.T) {
	b := &posthogBackend{client: nil}
	require.NotPanics(t, func() {
		b.TrackUp(&UpMetricsMetadata{success: true})
	})
}

func TestPostHogBackend_TrackUp_AnalyticsDisabled(t *testing.T) {
	teardown := setupPostHogContext(t, false)
	defer teardown()

	mock := &mockPostHogClient{}
	b := &posthogBackend{client: mock}
	b.TrackUp(&UpMetricsMetadata{success: true})

	require.Empty(t, mock.captured, "Enqueue must not be called when analytics is disabled")
}

func TestPostHogBackend_TrackUp_HappyPath(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	mock := &mockPostHogClient{}
	b := &posthogBackend{
		client:     mock,
		nsResolver: &mockNamespaceUIDResolver{uid: "ns-uid-789"},
	}

	b.TrackUp(&UpMetricsMetadata{
		success:          true,
		manifestType:     model.OktetoManifestType,
		isInteractive:    true,
		isBuildExecuted:  true,
		hasRunDeploy:     true,
		hasBuildSection:  true,
		hasDeploySection: true,
		service:          "api",
		namespace:        "dev-ns",
		repoURL:          "https://github.com/org/repo",
		execDuration:     90 * time.Second,
		isReconnect:      true,
		reconnectCount:   1,
		reconnectCause:   reconnectCauseDefault,
	})
	b.wg.Wait()

	require.Len(t, mock.captured, 1)
	ev := mock.captured[0]
	require.Equal(t, posthogUpEvent, ev.Event)
	require.Equal(t, "user-123", ev.DistinctId)
	require.Equal(t, true, ev.Properties["result"])
	require.Equal(t, "manifest", ev.Properties["manifest_type"])
	require.Equal(t, true, ev.Properties["is_interactive"])
	require.Equal(t, true, ev.Properties["is_build_executed"])
	require.Equal(t, true, ev.Properties["is_deploy_executed"])
	require.Equal(t, true, ev.Properties["has_build_section"])
	require.Equal(t, true, ev.Properties["has_deploy_section"])
	require.Equal(t, "api", ev.Properties["service"])
	require.Equal(t, "ns-uid-789", ev.Properties["namespace"])
	require.Equal(t, "bdb72e6e68b80f9ed3bbdb0ad1d2f8b4fac8ade379eb82182de40a3357a2d3b3", ev.Properties["repo_url"])
	require.Equal(t, 90, ev.Properties["duration_seconds"])
	require.Equal(t, true, ev.Properties["is_reconnect"])
	require.Equal(t, 1, ev.Properties["reconnect_count"])
	require.Equal(t, "unrecognised", ev.Properties["reconnect_cause"])
	require.NotContains(t, ev.Properties, "error_reason")

	// CLI common props
	require.NotEmpty(t, ev.Properties["cli_version"])
	require.NotEmpty(t, ev.Properties["os"])
	require.NotEmpty(t, ev.Properties["arch"])
	require.Equal(t, "test-machine", ev.Properties["machine_id"])
	require.Equal(t, "cli", ev.Properties["measurement_source"])

	// Common props
	require.Equal(t, "ACME Corp", ev.Properties["customer_name"])
	require.Equal(t, "cluster-uuid-1234", ev.Properties["cluster_id"])
	require.Equal(t, "1.2.3", ev.Properties["cluster_version"])
	require.Equal(t, "user-123", ev.Properties["user_id"])
}

func TestPostHogBackend_TrackUp_FailureIncludesErrorReason(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	mock := &mockPostHogClient{}
	b := &posthogBackend{client: mock}
	b.TrackUp(&UpMetricsMetadata{success: false, failActivate: true})
	b.wg.Wait()

	require.Len(t, mock.captured, 1)
	require.Equal(t, "fail_activate", mock.captured[0].Properties["error_reason"])
}

func TestPostHogBackend_TrackUpStarted_NilClient(t *testing.T) {
	b := &posthogBackend{client: nil}
	require.NotPanics(t, func() {
		b.TrackUpStarted("api", "dev-ns", "https://github.com/org/repo", "wf-123")
	})
}

func TestPostHogBackend_TrackUpStarted_AnalyticsDisabled(t *testing.T) {
	teardown := setupPostHogContext(t, false)
	defer teardown()

	mock := &mockPostHogClient{}
	b := &posthogBackend{client: mock}
	b.TrackUpStarted("api", "dev-ns", "https://github.com/org/repo", "wf-123")

	require.Empty(t, mock.captured, "Enqueue must not be called when analytics is disabled")
}

func TestPostHogBackend_TrackUpStarted_HappyPath(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	mock := &mockPostHogClient{}
	b := &posthogBackend{
		client:     mock,
		nsResolver: &mockNamespaceUIDResolver{uid: "ns-uid-456"},
	}
	b.TrackUpStarted("api", "dev-ns", "https://github.com/org/repo", "wf-abc-123")
	b.wg.Wait()

	require.Len(t, mock.captured, 1)
	ev := mock.captured[0]
	require.Equal(t, posthogUpStartedEvent, ev.Event)
	require.Equal(t, "user-123", ev.DistinctId)
	require.Equal(t, "api", ev.Properties["service"])
	require.Equal(t, "ns-uid-456", ev.Properties["namespace"])
	require.Equal(t, "bdb72e6e68b80f9ed3bbdb0ad1d2f8b4fac8ade379eb82182de40a3357a2d3b3", ev.Properties["repo_url"])
	require.Equal(t, "wf-abc-123", ev.Properties["workflow_id"])

	// CLI common props
	require.NotEmpty(t, ev.Properties["cli_version"])
	require.NotEmpty(t, ev.Properties["os"])
	require.NotEmpty(t, ev.Properties["arch"])
	require.Equal(t, "test-machine", ev.Properties["machine_id"])
	require.Equal(t, "cli", ev.Properties["measurement_source"])

	// Common props
	require.Equal(t, "ACME Corp", ev.Properties["customer_name"])
	require.Equal(t, "cluster-uuid-1234", ev.Properties["cluster_id"])
	require.Equal(t, "1.2.3", ev.Properties["cluster_version"])
	require.Equal(t, "user-123", ev.Properties["user_id"])
}

func TestPostHogBackend_TrackUpStarted_EmptyFieldsSentAsEmpty(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	mock := &mockPostHogClient{}
	b := &posthogBackend{client: mock}
	b.TrackUpStarted("", "", "", "")
	b.wg.Wait()

	require.Len(t, mock.captured, 1)
	ev := mock.captured[0]
	require.Equal(t, "", ev.Properties["service"])
	require.Equal(t, "", ev.Properties["namespace"])
	require.Equal(t, "", ev.Properties["repo_url"])
	require.Equal(t, "", ev.Properties["workflow_id"])
}

func TestPostHogBackend_TrackDeploy_NilClient(t *testing.T) {
	b := &posthogBackend{client: nil}
	require.NotPanics(t, func() {
		b.TrackDeploy(DeployMetadata{Success: true})
	})
}

func TestPostHogBackend_TrackDeploy_AnalyticsDisabled(t *testing.T) {
	teardown := setupPostHogContext(t, false)
	defer teardown()

	mock := &mockPostHogClient{}
	b := &posthogBackend{client: mock}
	b.TrackDeploy(DeployMetadata{Success: true})

	require.Empty(t, mock.captured, "Enqueue must not be called when analytics is disabled")
}

func TestPostHogBackend_TrackDeploy_HappyPath(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	mock := &mockPostHogClient{}
	b := &posthogBackend{
		client:     mock,
		nsResolver: &mockNamespaceUIDResolver{uid: "ns-uid-789"},
	}

	b.TrackDeploy(DeployMetadata{
		Success:                true,
		PipelineType:           model.OktetoManifestType,
		Namespace:              "dev-ns",
		WorkflowID:             "wf-deploy-1",
		Duration:               45 * time.Second,
		IsPreview:              false,
		IsRedeploy:             true,
		HasDependenciesSection: true,
		HasBuildSection:        true,
		IsRemote:               false,
		WaitForDependencies:    true,
	})
	b.wg.Wait()

	require.Len(t, mock.captured, 1)
	ev := mock.captured[0]
	require.Equal(t, posthogDeployCompletedEvent, ev.Event)
	require.Equal(t, "user-123", ev.DistinctId)
	require.Equal(t, true, ev.Properties["result"])
	require.Equal(t, "manifest", ev.Properties["manifest_archetype"])
	require.Equal(t, "wf-deploy-1", ev.Properties["workflow_id"])
	require.Equal(t, "ns-uid-789", ev.Properties["namespace"])
	require.Equal(t, float64(45), ev.Properties["duration_seconds"])
	require.Equal(t, false, ev.Properties["is_within_preview"])
	require.Equal(t, "regular", ev.Properties["namespace_type"])
	require.Equal(t, true, ev.Properties["is_redeploy"])
	require.Equal(t, true, ev.Properties["has_dependencies_section"])
	require.Equal(t, true, ev.Properties["has_build_section"])
	require.Equal(t, false, ev.Properties["is_remote"])
	require.Equal(t, true, ev.Properties["wait_for_dependencies"])
	require.NotContains(t, ev.Properties, "error_reason")
	require.Equal(t, "test-machine", ev.Properties["machine_id"])
	require.Equal(t, "ACME Corp", ev.Properties["customer_name"])
}

func TestPostHogBackend_TrackDeploy_WaitForDependenciesOmittedWhenFalse(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	mock := &mockPostHogClient{}
	b := &posthogBackend{client: mock}
	b.TrackDeploy(DeployMetadata{Success: true, WaitForDependencies: false})
	b.wg.Wait()

	require.Len(t, mock.captured, 1)
	require.NotContains(t, mock.captured[0].Properties, "wait_for_dependencies")
}

func TestPostHogBackend_TrackDeploy_FailureUnknownErrorOmitsErrorReason(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	mock := &mockPostHogClient{}
	b := &posthogBackend{client: mock}
	b.TrackDeploy(DeployMetadata{Success: false, Err: assert.AnError})
	b.wg.Wait()

	require.Len(t, mock.captured, 1)
	require.NotContains(t, mock.captured[0].Properties, "error_reason")
}

func TestPostHogBackend_TrackDeploy_KnownErrorsSetNormalizedErrorReason(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedReason string
	}{
		{
			name:           "no deploy commands",
			err:            oktetoErrors.ErrManifestFoundButNoDeployAndDependenciesCommands,
			expectedReason: "no_deploy_commands",
		},
		{
			name:           "timeout",
			err:            oktetoErrors.ErrTimeout,
			expectedReason: "timeout",
		},
		{
			name:           "command failed",
			err:            oktetoErrors.ErrCommandFailed,
			expectedReason: "command_failed",
		},
		{
			name:           "internal server error",
			err:            oktetoErrors.ErrInternalServerError,
			expectedReason: "internal_server_error",
		},
		{
			name:           "user error",
			err:            oktetoErrors.UserError{E: errors.New("something went wrong")},
			expectedReason: "user_error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			teardown := setupPostHogContext(t, true)
			defer teardown()

			mock := &mockPostHogClient{}
			b := &posthogBackend{client: mock}
			b.TrackDeploy(DeployMetadata{Success: false, Err: tt.err})
			b.wg.Wait()

			require.Len(t, mock.captured, 1)
			require.Equal(t, tt.expectedReason, mock.captured[0].Properties["error_reason"])
		})
	}
}

func TestPostHogBackend_TrackDeploy_NewFieldsHashedAndConditional(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	mock := &mockPostHogClient{}
	b := &posthogBackend{client: mock}
	b.TrackDeploy(DeployMetadata{
		Success:           true,
		PipelineType:      model.StackType,
		RepoURL:           "https://github.com/org/repo",
		ManifestSyntax:    "mixed",
		ParentExecutionID: "exec-parent-1",
		IsPreview:         true,
	})
	b.wg.Wait()

	require.Len(t, mock.captured, 1)
	ev := mock.captured[0]
	require.Equal(t, "compose", ev.Properties["manifest_archetype"])
	require.Equal(t, "mixed", ev.Properties["manifest_syntax"])
	require.Equal(t, "preview", ev.Properties["namespace_type"])
	require.Equal(t, true, ev.Properties["is_within_preview"])
	require.Equal(t, "exec-parent-1", ev.Properties["parent_execution_id"])
	require.Equal(t, "bdb72e6e68b80f9ed3bbdb0ad1d2f8b4fac8ade379eb82182de40a3357a2d3b3", ev.Properties["repo_url"])
}

func TestPostHogBackend_TrackDeploy_NewFieldsOmittedWhenEmpty(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	mock := &mockPostHogClient{}
	b := &posthogBackend{client: mock}
	b.TrackDeploy(DeployMetadata{Success: true})
	b.wg.Wait()

	require.Len(t, mock.captured, 1)
	ev := mock.captured[0]
	require.NotContains(t, ev.Properties, "repo_url")
	require.NotContains(t, ev.Properties, "manifest_syntax")
	require.NotContains(t, ev.Properties, "parent_execution_id")
	require.Equal(t, "pipeline", ev.Properties["manifest_archetype"])
	require.Equal(t, "regular", ev.Properties["namespace_type"])
}

func TestPostHogBackend_TrackDeployStarted_NilClient(t *testing.T) {
	b := &posthogBackend{client: nil}
	require.NotPanics(t, func() {
		b.TrackDeployStarted(DeployStartedMetadata{IsRedeploy: true})
	})
}

func TestPostHogBackend_TrackDeployStarted_AnalyticsDisabled(t *testing.T) {
	teardown := setupPostHogContext(t, false)
	defer teardown()

	mock := &mockPostHogClient{}
	b := &posthogBackend{client: mock}
	b.TrackDeployStarted(DeployStartedMetadata{IsRedeploy: true})

	require.Empty(t, mock.captured, "Enqueue must not be called when analytics is disabled")
}

func TestPostHogBackend_TrackDeployStarted_HappyPath(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	mock := &mockPostHogClient{}
	b := &posthogBackend{
		client:     mock,
		nsResolver: &mockNamespaceUIDResolver{uid: "ns-uid-654"},
	}
	b.TrackDeployStarted(DeployStartedMetadata{
		Namespace:  "dev-ns",
		RepoURL:    "https://github.com/org/repo",
		WorkflowID: "wf-deploy-1",
		IsPreview:  true,
		IsRedeploy: true,
	})
	b.wg.Wait()

	require.Len(t, mock.captured, 1)
	ev := mock.captured[0]
	require.Equal(t, posthogDeployStartedEvent, ev.Event)
	require.Equal(t, "user-123", ev.DistinctId)
	require.Equal(t, "ns-uid-654", ev.Properties["namespace"])
	require.Equal(t, "bdb72e6e68b80f9ed3bbdb0ad1d2f8b4fac8ade379eb82182de40a3357a2d3b3", ev.Properties["repo_url"])
	require.Equal(t, "wf-deploy-1", ev.Properties["workflow_id"])
	require.Equal(t, true, ev.Properties["is_within_preview"])
	require.Equal(t, true, ev.Properties["is_redeploy"])
	require.NotContains(t, ev.Properties, "ui_element")

	// Common props
	require.Equal(t, "ACME Corp", ev.Properties["customer_name"])
	require.Equal(t, "cli", ev.Properties["measurement_source"])
}

func TestPostHogBackend_TrackWakeTriggered_NilClient(t *testing.T) {
	b := &posthogBackend{client: nil}
	require.NotPanics(t, func() {
		b.TrackWakeTriggered(context.Background(), WakeTriggeredMetadata{Namespace: "dev-ns"})
	})
}

func TestPostHogBackend_TrackWakeTriggered_AnalyticsDisabled(t *testing.T) {
	teardown := setupPostHogContext(t, false /* disabled */)
	defer teardown()

	mock := &mockPostHogClient{}
	b := &posthogBackend{client: mock}
	b.TrackWakeTriggered(context.Background(), WakeTriggeredMetadata{Namespace: "dev-ns"})

	require.Empty(t, mock.captured, "Enqueue must not be called when analytics is disabled")
}

func TestPostHogBackend_TrackWakeTriggered_HappyPath(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	mock := &mockPostHogClient{}
	b := &posthogBackend{
		client:     mock,
		nsResolver: &mockNamespaceUIDResolver{uid: "ns-uid-wake"},
	}
	b.TrackWakeTriggered(context.Background(), WakeTriggeredMetadata{
		Namespace: "dev-ns",
		IsPreview: true,
	})
	b.wg.Wait()

	require.Len(t, mock.captured, 1)
	ev := mock.captured[0]
	require.Equal(t, posthogWakeTriggeredEvent, ev.Event)
	require.Equal(t, "user-123", ev.DistinctId)
	require.Equal(t, "ns-uid-wake", ev.Properties["namespace"])
	require.Equal(t, true, ev.Properties["is_preview"])
	require.NotContains(t, ev.Properties, "workflow_id")
	require.NotContains(t, ev.Properties, "ui_element")

	// Common props
	require.Equal(t, "ACME Corp", ev.Properties["customer_name"])
	require.Equal(t, "cli", ev.Properties["measurement_source"])
	require.Equal(t, "user-123", ev.Properties["user_id"])
}
