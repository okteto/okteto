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

	"github.com/dukex/mixpanel"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/require"
)

var fixedEventTime = time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)

type sentEvent struct {
	timestamp  *time.Time
	props      map[string]any
	distinctID string
	event      string
}

// fakeEventTracker records the events sent to Mixpanel. When block is set,
// Track waits on it before recording, to simulate a slow network.
type fakeEventTracker struct {
	err    error
	block  chan struct{}
	events []sentEvent
	mu     sync.Mutex
}

func (f *fakeEventTracker) Track(distinctID, eventName string, e *mixpanel.Event) error {
	if f.block != nil {
		<-f.block
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, sentEvent{distinctID: distinctID, event: eventName, timestamp: e.Timestamp, props: e.Properties})
	return f.err
}

func (f *fakeEventTracker) sent() []sentEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]sentEvent(nil), f.events...)
}

type mockNamespaceUIDResolver struct {
	err error
	uid string
}

func (m *mockNamespaceUIDResolver) GetNamespaceUID(_ context.Context, _ string) (string, error) {
	return m.uid, m.err
}

func newTestEventClient(tracker eventTracker, resolver NamespaceUIDResolver) *productEventClient {
	return &productEventClient{
		client:       tracker,
		nsResolver:   resolver,
		now:          func() time.Time { return fixedEventTime },
		closeTimeout: 5 * time.Second,
	}
}

// setupAnalyticsContext installs an initialized Okteto context and analytics
// config for the duration of the test.
func setupAnalyticsContext(t *testing.T, analyticsEnabled bool) {
	t.Helper()
	setupAnalyticsContextWithUser(t, analyticsEnabled, "user-123")
}

func setupAnalyticsContextWithUser(t *testing.T, analyticsEnabled bool, userID string) {
	t.Helper()
	prevAnalytics := currentAnalytics
	prevStore := okteto.CurrentStore
	prevVersion := config.VersionString
	t.Cleanup(func() {
		currentAnalytics = prevAnalytics
		okteto.CurrentStore = prevStore
		config.VersionString = prevVersion
	})

	currentAnalytics = &Analytics{Enabled: analyticsEnabled, MachineID: "test-machine"}
	config.VersionString = "test-version"
	okteto.CurrentStore = &okteto.ContextStore{
		CurrentContext: "https://okteto.example.com",
		Contexts: map[string]*okteto.Context{
			"https://okteto.example.com": {
				Name:           "https://okteto.example.com",
				UserID:         userID,
				CompanyName:    "ACME Corp",
				ClusterID:      "cluster-uuid-1234",
				ClusterVersion: "1.2.3",
				Analytics:      true,
			},
		},
	}
}

func TestProductEventClient_Track_NilClientDoesNotPanic(t *testing.T) {
	var c *productEventClient
	require.NotPanics(t, func() {
		c.track(context.Background(), "test_event", nil, c.withNamespace("ns"))
		c.Close()
	})
}

func TestProductEventClient_Track_AnalyticsDisabled(t *testing.T) {
	setupAnalyticsContext(t, false)
	fake := &fakeEventTracker{}
	c := newTestEventClient(fake, nil)

	c.track(context.Background(), "test_event", map[string]any{})
	c.Close()

	require.Empty(t, fake.sent())
}

func TestProductEventClient_Track_ContextNotInitialized(t *testing.T) {
	prevAnalytics := currentAnalytics
	prevStore := okteto.CurrentStore
	t.Cleanup(func() {
		currentAnalytics = prevAnalytics
		okteto.CurrentStore = prevStore
	})
	currentAnalytics = &Analytics{Enabled: true}
	okteto.CurrentStore = &okteto.ContextStore{CurrentContext: ""}
	fake := &fakeEventTracker{}
	c := newTestEventClient(fake, nil)

	c.track(context.Background(), "test_event", map[string]any{})
	c.Close()

	require.Empty(t, fake.sent())
}

func TestProductEventClient_Track_DropsEventWithoutUserID(t *testing.T) {
	setupAnalyticsContextWithUser(t, true, "")
	fake := &fakeEventTracker{}
	c := newTestEventClient(fake, nil)

	c.track(context.Background(), "test_event", map[string]any{})
	c.Close()

	require.Empty(t, fake.sent())
}

func TestProductEventClient_Track_SendsCommonAndEventProperties(t *testing.T) {
	setupAnalyticsContext(t, true)
	t.Setenv("OKTETO_ORIGIN", "")
	t.Setenv("OKTETO_WITHIN_DEPLOY_COMMAND_CONTEXT", "")
	fake := &fakeEventTracker{}
	c := newTestEventClient(fake, nil)

	c.track(context.Background(), "test_event", map[string]any{"key": "val"})
	c.Close()

	events := fake.sent()
	require.Len(t, events, 1)
	ev := events[0]
	require.Equal(t, "test_event", ev.event)
	require.Equal(t, "user-123", ev.distinctID)
	require.Equal(t, fixedEventTime, *ev.timestamp)
	require.Equal(t, "val", ev.props["key"])
	require.Equal(t, "ACME Corp", ev.props["customer_name"])
	require.Equal(t, "cluster-uuid-1234", ev.props["cluster_id"])
	require.Equal(t, "https://okteto.example.com", ev.props["cluster_url"])
	require.Equal(t, "1.2.3", ev.props["cluster_version"])
	require.Equal(t, "user-123", ev.props["user_id"])
	require.Equal(t, "test-version", ev.props["cli_version"])
	require.NotEmpty(t, ev.props["os"])
	require.NotEmpty(t, ev.props["arch"])
	require.Equal(t, "test-machine", ev.props["machine_id"])
	require.Equal(t, "cli", ev.props["measurement_source"])
	require.Equal(t, "cli", ev.props["trigger_source"])
}

// The product events keep PostHog's properties: they must not get the default
// properties track() adds to the other Mixpanel events.
func TestProductEventClient_Track_DoesNotAddTrackDefaults(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{}
	c := newTestEventClient(fake, nil)

	c.track(context.Background(), "test_event", map[string]any{})
	c.Close()

	events := fake.sent()
	require.Len(t, events, 1)
	for _, key := range []string{"success", "origin", "source", "version", "context", "$os", "contextType", "isOkteto", "term-type", "isTrial"} {
		require.NotContains(t, events[0].props, key)
	}
}

func TestProductEventClient_Track_EventPropertiesOverrideCommon(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{}
	c := newTestEventClient(fake, nil)

	c.track(context.Background(), "test_event", map[string]any{"cluster_id": "override"})
	c.Close()

	require.Equal(t, "override", fake.sent()[0].props["cluster_id"])
}

func TestProductEventClient_Track_AppliesEnrichersBeforeSending(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{}
	c := newTestEventClient(fake, nil)
	enricher := func(_ context.Context, props map[string]any) {
		props["enriched_key"] = "enriched_value"
	}

	c.track(context.Background(), "test_event", map[string]any{}, enricher)
	c.Close()

	events := fake.sent()
	require.Len(t, events, 1)
	require.Equal(t, "enriched_value", events[0].props["enriched_key"])
}

func TestProductEventClient_Track_SendErrorIsNotFatal(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{err: errors.New("mixpanel unavailable")}
	c := newTestEventClient(fake, nil)

	require.NotPanics(t, func() {
		c.track(context.Background(), "test_event", map[string]any{})
		c.Close()
	})
	require.Len(t, fake.sent(), 1)
}

func TestProductEventClient_Track_DoesNotBlockCaller(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{block: make(chan struct{})}
	c := newTestEventClient(fake, nil)

	c.track(context.Background(), "test_event", map[string]any{})

	require.Empty(t, fake.sent(), "the event is sent in the background")
	close(fake.block)
	c.Close()
	require.Len(t, fake.sent(), 1)
}

func TestProductEventClient_Track_DropsEventsAfterClose(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{}
	c := newTestEventClient(fake, nil)

	c.Close()
	c.track(context.Background(), "test_event", map[string]any{})
	c.wg.Wait()

	require.Empty(t, fake.sent())
}

func TestProductEventClient_Close_WaitsForPendingEvents(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{}
	c := newTestEventClient(fake, nil)

	for range 10 {
		c.track(context.Background(), "test_event", map[string]any{})
	}
	c.Close()

	require.Len(t, fake.sent(), 10)
}

func TestProductEventClient_Close_TimesOut(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{block: make(chan struct{})}
	t.Cleanup(func() { close(fake.block) })
	c := newTestEventClient(fake, nil)
	c.closeTimeout = 50 * time.Millisecond

	c.track(context.Background(), "test_event", map[string]any{})
	start := time.Now()
	c.Close()

	require.Less(t, time.Since(start), 5*time.Second)
	require.Empty(t, fake.sent())
}

func TestProductEventClient_TrackDuringClose(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{}
	c := newTestEventClient(fake, nil)

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.track(context.Background(), "test_event", map[string]any{})
		}()
	}
	c.Close()
	sentAtClose := len(fake.sent())
	wg.Wait()
	c.wg.Wait()

	// Close returned only after every accepted event was sent; the rest were dropped.
	require.Len(t, fake.sent(), sentAtClose)
}

func TestProductEventClient_Track_NilMixpanelClient(t *testing.T) {
	setupAnalyticsContext(t, true)
	c := newTestEventClient(nil, nil)

	require.NotPanics(t, func() {
		c.track(context.Background(), "test_event", map[string]any{})
		c.Close()
	})
}

func TestProductEventClient_AgentType_OmittedWhenNoAgent(t *testing.T) {
	setupAnalyticsContext(t, true)
	t.Setenv("CODEX_CI", "")
	t.Setenv("GEMINI_CLI", "")
	t.Setenv("CLAUDECODE", "")
	t.Setenv("CURSOR_SANDBOX", "")

	props := commonProductEventProperties()

	require.NotContains(t, props, "agent_type")
	require.Equal(t, false, props["is_agent"])
}

func TestProductEventClient_AgentType_PresentWhenAgent(t *testing.T) {
	setupAnalyticsContext(t, true)
	t.Setenv("CODEX_CI", "")
	t.Setenv("GEMINI_CLI", "")
	t.Setenv("CLAUDECODE", "true")
	t.Setenv("CURSOR_SANDBOX", "")

	props := commonProductEventProperties()

	require.Equal(t, true, props["is_agent"])
	require.Equal(t, "claude_code", props["agent_type"])
}

func TestCommonProductEventProperties_TriggerSource(t *testing.T) {
	tests := []struct {
		name                string
		origin              string
		withinDeployContext string
		expected            string
	}{
		{name: "from origin", origin: "web", expected: "web"},
		{name: "defaults to cli", expected: "cli"},
		{name: "within deploy context", withinDeployContext: "true", expected: "okteto-deploy"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupAnalyticsContext(t, true)
			t.Setenv("OKTETO_ORIGIN", tt.origin)
			t.Setenv("OKTETO_WITHIN_DEPLOY_COMMAND_CONTEXT", tt.withinDeployContext)

			require.Equal(t, tt.expected, commonProductEventProperties()["trigger_source"])
		})
	}
}

// is_automation is true when the actor is a CI run or an AI agent, and false for a manual human run.
func TestCommonProductEventProperties_Automation(t *testing.T) {
	tests := []struct {
		name         string
		ci           string
		claudeCode   string
		isCI         bool
		isAgent      bool
		isAutomation bool
	}{
		{name: "manual run"},
		{name: "ci", ci: "true", isCI: true, isAutomation: true},
		{name: "agent", claudeCode: "true", isAgent: true, isAutomation: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupAnalyticsContext(t, true)
			t.Setenv("CI", tt.ci)
			t.Setenv("CLAUDECODE", tt.claudeCode)
			t.Setenv("CODEX_CI", "")
			t.Setenv("GEMINI_CLI", "")
			t.Setenv("CURSOR_SANDBOX", "")

			props := commonProductEventProperties()

			require.Equal(t, tt.isCI, props["is_ci"])
			require.Equal(t, tt.isAgent, props["is_agent"])
			require.Equal(t, tt.isAutomation, props["is_automation"])
		})
	}
}

func TestProductEventClient_WithNamespaceUID(t *testing.T) {
	tests := []struct {
		resolver NamespaceUIDResolver
		name     string
		nsName   string
		expected string
	}{
		{name: "adds the UID", resolver: &mockNamespaceUIDResolver{uid: "ns-uid-abc"}, nsName: "my-ns", expected: "ns-uid-abc"},
		{name: "empty namespace", resolver: &mockNamespaceUIDResolver{uid: "ns-uid-abc"}, nsName: "", expected: ""},
		{name: "nil resolver", resolver: nil, nsName: "my-ns", expected: ""},
		{name: "resolver error", resolver: &mockNamespaceUIDResolver{err: errors.New("k8s unavailable")}, nsName: "my-ns", expected: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &productEventClient{nsResolver: tt.resolver}

			nsProps := map[string]any{}
			c.withNamespace(tt.nsName)(context.Background(), nsProps)
			previewProps := map[string]any{}
			c.withPreview(tt.nsName)(context.Background(), previewProps)

			require.Equal(t, map[string]any{"namespace": tt.expected}, nsProps)
			require.Equal(t, map[string]any{"preview": tt.expected}, previewProps)
		})
	}
}

func TestProductEventClient_WithNamespaceUID_WritesGivenKey(t *testing.T) {
	c := &productEventClient{nsResolver: &mockNamespaceUIDResolver{uid: "uid-9"}}

	props := map[string]any{}
	c.withNamespaceUID("custom_key", "some-ns")(context.Background(), props)

	require.Equal(t, "uid-9", props["custom_key"])
}
