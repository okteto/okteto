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
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/posthog/posthog-go"
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
	require.Equal(t, "claude", mock.captured[0].Properties["agent_type"])
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
