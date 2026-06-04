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
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/posthog/posthog-go"
	"github.com/stretchr/testify/require"
)

// mockPostHogClient captures Enqueue calls for assertion.
type mockPostHogClient struct {
	captured []posthog.Capture
	err      error
}

func (m *mockPostHogClient) Enqueue(msg posthog.Message) error {
	if c, ok := msg.(posthog.Capture); ok {
		m.captured = append(m.captured, c)
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
	require.Equal(t, "bdb72e6e68b80f9ed3bbdb0ad1d2f8b4fac8ade379eb82182de40a3357a2d3b3", event.Properties["repo_url"])
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
	require.Equal(t, "claude", mock.captured[0].Properties["agent_type"])
}

