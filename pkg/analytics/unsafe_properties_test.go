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
	"maps"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDropUnsafeProperties_KeepsSafeValues(t *testing.T) {
	props := map[string]any{
		"namespace":    testNamespaceUID,
		"preview":      strings.ToUpper(testPreviewUID),
		"repoURL":      hashString("https://github.com/okteto/okteto"),
		"name":         hashString("api"),
		"service":      hashString("api"),
		"errorReason":  "dockerfile_failed",
		"error":        "command_failed",
		"customerName": "ACME Corp",
	}
	expected := maps.Clone(props)

	dropUnsafeProperties("event", props)

	require.Equal(t, expected, props)
}

func TestDropUnsafeProperties_KeepsEmptyValues(t *testing.T) {
	props := map[string]any{
		"namespace":   "",
		"preview":     "",
		"repoURL":     "",
		"name":        "",
		"service":     "",
		"errorReason": "",
		"error":       "",
	}

	dropUnsafeProperties("event", props)

	require.Equal(t, map[string]any{
		"namespace":   "",
		"preview":     "",
		"repoURL":     "",
		"name":        "",
		"service":     "",
		"errorReason": "",
		"error":       "",
	}, props)
}

func TestDropUnsafeProperties_LeavesMissingPropertiesOut(t *testing.T) {
	props := map[string]any{"isPreview": true}

	dropUnsafeProperties("event", props)

	require.Equal(t, map[string]any{"isPreview": true}, props)
}

func TestDropUnsafeProperties_ReplacesUnsafeValues(t *testing.T) {
	tests := []struct {
		value    any
		name     string
		key      string
		expected string
	}{
		{name: "namespace name", key: "namespace", value: "my-namespace", expected: ""},
		{name: "non-string namespace", key: "namespace", value: 42, expected: ""},
		{name: "preview name", key: "preview", value: "pr-123", expected: ""},
		{name: "raw repo URL with credentials", key: "repoURL", value: "https://user:secret@github.com/org/repo.git", expected: ""},
		{name: "SHA-1 repoURL", key: "repoURL", value: "2fd4e1c67a2d28fced849ee1bb76e7391b93eb12", expected: ""},
		{name: "uppercase SHA-256 repoURL", key: "repoURL", value: strings.ToUpper(hashString("repo")), expected: ""},
		{name: "raw image name", key: "name", value: "api", expected: ""},
		{name: "raw service name", key: "service", value: "frontend", expected: ""},
		{name: "raw error with credentials", key: "errorReason", value: "failed to pull https://user:secret@registry.example.com/app", expected: "unknown"},
		{name: "too long category", key: "errorReason", value: strings.Repeat("a", 65), expected: "unknown"},
		{name: "non-string errorReason", key: "errorReason", value: true, expected: "unknown"},
		{name: "raw deploy error", key: "error", value: "exit status 1: kubectl apply -f k8s.yml", expected: "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			props := map[string]any{tt.key: tt.value}

			dropUnsafeProperties("event", props)

			require.Equal(t, map[string]any{tt.key: tt.expected}, props)
		})
	}
}

// The error categories the CLI sends must fit the category shape, or they'd be sent as "unknown".
func TestDropUnsafeProperties_KeepsErrorCategories(t *testing.T) {
	categories := []string{
		// imageBuild.errorReason (cmd/build/v2/error_reason.go)
		"queue_timeout", "connection_failure", "registry_error", "image_not_found",
		"dockerfile_missing", "dockerfile_failed", "secret_error", "unknown",
		// Up.errorReason
		"fail_activate", "err_sync_insufficient_space", "err_sync_reset_database", "err_sync_lost_syncthing", "err_sync",
		// Deploy.error
		"no_deploy_commands", "timeout", "command_failed", "internal_server_error", "user_error",
	}
	for _, category := range categories {
		t.Run(category, func(t *testing.T) {
			props := map[string]any{"errorReason": category, "error": category}

			dropUnsafeProperties("event", props)

			require.Equal(t, map[string]any{"errorReason": category, "error": category}, props)
		})
	}
}

func TestProductEventClient_Track_DropsUnsafeEnrichedValue(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{}
	c := newTestEventClient(fake, &mockNamespaceUIDResolver{uid: "dev-namespace"})

	c.track(context.Background(), productUpStartedEvent, map[string]any{}, c.withNamespace("dev-namespace"))
	c.Close()

	events := fake.sent()
	require.Len(t, events, 1)
	require.Equal(t, "", events[0].props["namespace"], "a namespace name is never sent")
}

func TestProductEventClient_TrackWithProductProperties_DropsUnsafeValue(t *testing.T) {
	setupAnalyticsContext(t, true)
	fake := &fakeEventTracker{}
	c := newTestEventClient(fake, nil)

	c.trackWithProductProperties(context.Background(), "Merged", true, map[string]any{"repoURL": "https://github.com/org/repo"})
	c.Close()

	events := fake.sent()
	require.Len(t, events, 1)
	require.Equal(t, "", events[0].props["repoURL"], "a raw repo URL is never sent")
	require.Equal(t, "ACME Corp", events[0].props["customerName"], "the other properties are sent as they are")
}
