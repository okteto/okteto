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

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/posthog/posthog-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPostHogClient captures Enqueue calls for assertion.
type mockPostHogClient struct {
	captured       []posthog.Capture
	capturedGroups []posthog.GroupIdentify
	err            error
}

func (m *mockPostHogClient) Enqueue(msg posthog.Message) error {
	if c, ok := msg.(posthog.Capture); ok {
		m.captured = append(m.captured, c)
	}
	if gi, ok := msg.(posthog.GroupIdentify); ok {
		m.capturedGroups = append(m.capturedGroups, gi)
	}
	return m.err
}

func (m *mockPostHogClient) Close() error { return nil }

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

	mock := &mockPostHogClient{}
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

	require.Len(t, mock.captured, 1)
	event := mock.captured[0]
	require.Equal(t, posthogImageBuildEvent, event.Event)
	require.Equal(t, "user-123", event.DistinctId)

	// Event-specific props
	require.Equal(t, "api", event.Properties["service"])
	require.Equal(t, 30, event.Properties["duration_seconds"])
	require.Equal(t, 5, event.Properties["queue_duration_seconds"])
	require.Equal(t, int64(3000), event.Properties["build_context_duration_ms"])
	require.Equal(t, true, event.Properties["result"])
	require.Equal(t, int64(20_000_000), event.Properties["build_context_size_bytes"])
	require.Equal(t, false, event.Properties["is_cache"])
	require.Equal(t, "proxy", event.Properties["connection_type"])
	require.Equal(t, "https://github.com/org/repo", event.Properties["repo_url"])
	require.NotContains(t, event.Properties, "error_reason")

	// CLI common props
	require.NotEmpty(t, event.Properties["cli_version"])
	require.NotEmpty(t, event.Properties["os"])
	require.NotEmpty(t, event.Properties["arch"])
	require.Equal(t, "test-machine", event.Properties["machine_id"])

	// Common props
	require.Equal(t, "ACME Corp", event.Properties["customer_name"])
	require.Equal(t, "cluster-uuid-1234", event.Properties["cluster_id"])
	require.Equal(t, "1.2.3", event.Properties["cluster_version"])
	require.Equal(t, "user-123", event.Properties["user_id"])

	// Groups
	require.Equal(t, "ACME Corp", event.Groups["customer"])
	require.Equal(t, "cluster-uuid-1234", event.Groups["cluster"])
}

func TestPostHogBackend_TrackImageBuild_EnqueueError(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	mock := &mockPostHogClient{err: errors.New("posthog unavailable")}
	b := &posthogBackend{client: mock}

	// Must not panic — error is logged and swallowed
	require.NotPanics(t, func() {
		b.TrackImageBuild(context.Background(), &ImageBuildMetadata{Success: true})
	})
}

func TestPostHogBackend_IdentifyGroups_NilClient(t *testing.T) {
	b := &posthogBackend{client: nil}
	// Must not panic
	require.NotPanics(t, func() { b.IdentifyGroups() })
}

func TestPostHogBackend_IdentifyGroups_AnalyticsDisabled(t *testing.T) {
	teardown := setupPostHogContext(t, false)
	defer teardown()

	mock := &mockPostHogClient{}
	b := &posthogBackend{client: mock}
	b.IdentifyGroups()

	require.Empty(t, mock.capturedGroups, "GroupIdentify must not be sent when analytics is disabled")
}

func TestPostHogBackend_AgentType_OmittedWhenNoAgent(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	// Unset all agent env vars so getAgent() returns ""
	t.Setenv("CODEX_CI", "")
	t.Setenv("GEMINI_CLI", "")
	t.Setenv("CLAUDECODE", "")
	t.Setenv("CURSOR_SANDBOX", "")

	mock := &mockPostHogClient{}
	b := &posthogBackend{client: mock}
	b.TrackImageBuild(context.Background(), &ImageBuildMetadata{Success: true})

	require.Len(t, mock.captured, 1)
	require.Equal(t, false, mock.captured[0].Properties["is_agent"])
	require.NotContains(t, mock.captured[0].Properties, "agent_type")
}

func TestPostHogBackend_AgentType_PresentWhenAgent(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	t.Setenv("CODEX_CI", "")
	t.Setenv("GEMINI_CLI", "")
	t.Setenv("CLAUDECODE", "true")
	t.Setenv("CURSOR_SANDBOX", "")

	mock := &mockPostHogClient{}
	b := &posthogBackend{client: mock}
	b.TrackImageBuild(context.Background(), &ImageBuildMetadata{Success: true})

	require.Len(t, mock.captured, 1)
	require.Equal(t, true, mock.captured[0].Properties["is_agent"])
	require.Equal(t, "claude_code", mock.captured[0].Properties["agent_type"])
}

func TestPostHogBackend_IdentifyGroups_HappyPath(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	mock := &mockPostHogClient{}
	b := &posthogBackend{client: mock}
	b.IdentifyGroups()

	require.Len(t, mock.capturedGroups, 2)

	clusterMsg := mock.capturedGroups[0]
	require.Equal(t, "cluster", clusterMsg.Type)
	require.Equal(t, "cluster-uuid-1234", clusterMsg.Key)
	require.Equal(t, "cluster-uuid-1234", clusterMsg.Properties["cluster_id"])

	customerMsg := mock.capturedGroups[1]
	require.Equal(t, "customer", customerMsg.Type)
	require.Equal(t, "ACME Corp", customerMsg.Key)
	require.Equal(t, "ACME Corp", customerMsg.Properties["customer_id"])
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
	b := &posthogBackend{client: mock}

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
	require.Equal(t, "dev-ns", ev.Properties["namespace"])
	require.Equal(t, "https://github.com/org/repo", ev.Properties["repo_url"])
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

	// Groups
	require.Equal(t, "ACME Corp", ev.Groups["customer"])
	require.Equal(t, "cluster-uuid-1234", ev.Groups["cluster"])
}

func TestPostHogBackend_TrackUp_FailureIncludesErrorReason(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	mock := &mockPostHogClient{}
	b := &posthogBackend{client: mock}
	b.TrackUp(&UpMetricsMetadata{success: false, failActivate: true})

	require.Len(t, mock.captured, 1)
	require.Equal(t, "fail_activate", mock.captured[0].Properties["error_reason"])
}

func TestPostHogBackend_TrackUpStarted_NilClient(t *testing.T) {
	b := &posthogBackend{client: nil}
	require.NotPanics(t, func() {
		b.TrackUpStarted("api", "dev-ns", "https://github.com/org/repo")
	})
}

func TestPostHogBackend_TrackUpStarted_AnalyticsDisabled(t *testing.T) {
	teardown := setupPostHogContext(t, false)
	defer teardown()

	mock := &mockPostHogClient{}
	b := &posthogBackend{client: mock}
	b.TrackUpStarted("api", "dev-ns", "https://github.com/org/repo")

	require.Empty(t, mock.captured, "Enqueue must not be called when analytics is disabled")
}

func TestPostHogBackend_TrackUpStarted_HappyPath(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	mock := &mockPostHogClient{}
	b := &posthogBackend{client: mock}
	b.TrackUpStarted("api", "dev-ns", "https://github.com/org/repo")

	require.Len(t, mock.captured, 1)
	ev := mock.captured[0]
	require.Equal(t, posthogUpStartedEvent, ev.Event)
	require.Equal(t, "user-123", ev.DistinctId)
	require.Equal(t, "api", ev.Properties["service"])
	require.Equal(t, "dev-ns", ev.Properties["namespace"])
	require.Equal(t, "https://github.com/org/repo", ev.Properties["repo_url"])

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

	// Groups
	require.Equal(t, "ACME Corp", ev.Groups["customer"])
	require.Equal(t, "cluster-uuid-1234", ev.Groups["cluster"])
}

func TestPostHogBackend_TrackUpStarted_OmitsEmptyFields(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	mock := &mockPostHogClient{}
	b := &posthogBackend{client: mock}
	b.TrackUpStarted("", "", "")

	require.Len(t, mock.captured, 1)
	ev := mock.captured[0]
	require.NotContains(t, ev.Properties, "service")
	require.NotContains(t, ev.Properties, "namespace")
	require.NotContains(t, ev.Properties, "repo_url")
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
	b := &posthogBackend{client: mock}

	b.TrackDeploy(DeployMetadata{
		Success:                true,
		PipelineType:           model.OktetoManifestType,
		Namespace:              "dev-ns",
		Duration:               45 * time.Second,
		IsPreview:              false,
		IsRedeploy:             true,
		HasDependenciesSection: true,
		HasBuildSection:        true,
		IsRemote:               false,
		WaitForDependencies:    true,
	})

	require.Len(t, mock.captured, 1)
	ev := mock.captured[0]
	require.Equal(t, posthogDeployCompletedEvent, ev.Event)
	require.Equal(t, "user-123", ev.DistinctId)
	require.Equal(t, true, ev.Properties["result"])
	require.Equal(t, "manifest", ev.Properties["deploy_type"])
	require.Equal(t, "dev-ns", ev.Properties["namespace"])
	require.Equal(t, 45, ev.Properties["duration_seconds"])
	require.Equal(t, false, ev.Properties["is_preview"])
	require.Equal(t, true, ev.Properties["is_redeploy"])
	require.Equal(t, true, ev.Properties["has_dependencies_section"])
	require.Equal(t, true, ev.Properties["has_build_section"])
	require.Equal(t, false, ev.Properties["is_remote"])
	require.Equal(t, true, ev.Properties["wait_for_dependencies"])
	require.NotContains(t, ev.Properties, "error_reason")
	require.Equal(t, "test-machine", ev.Properties["machine_id"])
	require.Equal(t, "ACME Corp", ev.Properties["customer_id"])
	require.Equal(t, "ACME Corp", ev.Groups["customer"])
}

func TestPostHogBackend_TrackDeploy_FailureIncludesErrorReason(t *testing.T) {
	teardown := setupPostHogContext(t, true)
	defer teardown()

	mock := &mockPostHogClient{}
	b := &posthogBackend{client: mock}
	b.TrackDeploy(DeployMetadata{Success: false, Err: assert.AnError})

	require.Len(t, mock.captured, 1)
	require.Equal(t, assert.AnError.Error(), mock.captured[0].Properties["error_reason"])
}
