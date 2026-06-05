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

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAnalyticsTracker_TrackImageBuild(t *testing.T) {
	tests := []struct {
		input           *ImageBuildMetadata
		expectedSuccess bool
		name            string
	}{
		{
			name:            "success event",
			input:           &ImageBuildMetadata{Success: true},
			expectedSuccess: true,
		},
		{
			name:            "not success event",
			input:           &ImageBuildMetadata{Success: false},
			expectedSuccess: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedMeta *ImageBuildMetadata
			mock := &mockAnalyticsBackend{
				trackImageBuildFn: func(_ context.Context, m *ImageBuildMetadata) {
					capturedMeta = m
				},
			}
			tracker := &Tracker{backends: []analyticsBackend{mock}}
			tracker.TrackImageBuild(context.Background(), tt.input)

			require.NotNil(t, capturedMeta)
			require.Equal(t, tt.expectedSuccess, capturedMeta.Success)
		})
	}
}

func Test_ImageBuildMetadata_toMixpanelProps(t *testing.T) {
	m := &ImageBuildMetadata{
		Name:                     "test-service",
		RepoURL:                  "http://myrepo.url",
		CacheHit:                 true,
		CacheHitDuration:         5 * time.Second,
		BuildDuration:            5 * time.Second,
		WaitForBuildkitAvailable: 12 * time.Second,
		BuildContextHash:         "contextHash",
		BuildContextHashDuration: 5 * time.Second,
		Initiator:                "me",
	}

	expectedProps := map[string]any{
		"name":                            "665653223b1e8bfa2d462b3adb06d49f8984052e5df03d7fd2365293a102fce8",
		"repoURL":                         "82eec095b1cc767833c5e4b5d7b02a6df10c0f284127c7e840e1f460b1896067",
		"cacheHit":                        true,
		"cacheHitDurationSeconds":         float64(5),
		"waitForBuildkitAvailable":        float64(12),
		"buildDurationSeconds":            float64(5),
		"cloneDurationSeconds":            float64(0),
		"buildContextHash":                "contextHash",
		"buildContextHashDurationSeconds": float64(5),
		"initiator":                       "me",
	}

	require.Equal(t, expectedProps, m.toMixpanelProps())
}

func Test_NewImageBuildMetadata(t *testing.T) {
	require.Empty(t, NewImageBuildMetadata())
	require.IsType(t, &ImageBuildMetadata{}, NewImageBuildMetadata())
}

func Test_ImageBuildMetadata_toPostHogProps(t *testing.T) {
	m := &ImageBuildMetadata{
		Name:                     "api",
		Success:                  true,
		BuildDuration:            30 * time.Second,
		WaitForBuildkitAvailable: 5 * time.Second,
		BuildkitDuration:         25 * time.Second,
		ContextTransferDuration:  3 * time.Second,
		BuildContextSize:         20_000_000,
		CacheHit:                 true,
		ConnectionType:           "proxy",
		RepoURL:                  "https://github.com/org/repo",
	}

	props := m.toPostHogProps()

	require.Equal(t, "api", props["service"])
	require.Equal(t, 30, props["duration_seconds"])
	require.Equal(t, 5, props["queue_duration_seconds"])
	require.Equal(t, int64(3000), props["build_context_duration_ms"])
	require.Equal(t, true, props["result"])
	require.Equal(t, int64(20_000_000), props["build_context_size_bytes"])
	require.Equal(t, true, props["is_cache"])
	require.Equal(t, "proxy", props["connection_type"])
	require.Equal(t, "bdb72e6e68b80f9ed3bbdb0ad1d2f8b4fac8ade379eb82182de40a3357a2d3b3", props["repo_url"])
	require.NotContains(t, props, "error_reason", "error_reason must be omitted on success")
}

func Test_ImageBuildMetadata_toPostHogProps_omitsZeroFields(t *testing.T) {
	m := &ImageBuildMetadata{
		Name:    "api",
		Success: true,
	}

	props := m.toPostHogProps()

	require.NotContains(t, props, "duration_seconds")
	require.NotContains(t, props, "queue_duration_seconds")
	require.NotContains(t, props, "build_context_duration_ms")
	require.NotContains(t, props, "build_context_size_bytes")
	require.NotContains(t, props, "connection_type")
	require.NotContains(t, props, "repo_url")
	require.NotContains(t, props, "error_reason")
}

func Test_ImageBuildMetadata_toPostHogProps_withError(t *testing.T) {
	m := &ImageBuildMetadata{
		Success:     false,
		ErrorReason: "registry_pull_error",
	}

	props := m.toPostHogProps()

	require.Equal(t, false, props["result"])
	require.Equal(t, "registry_pull_error", props["error_reason"])
}
