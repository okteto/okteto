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
		input         *ImageBuildMetadata
		expectedEvent *mockEvent
		name          string
	}{
		{
			name: "success event",
			input: &ImageBuildMetadata{
				Success: true,
			},
			expectedEvent: &mockEvent{
				event:   "imageBuild",
				success: true,
				props: map[string]interface{}{
					"name":                            "",
					"repoURL":                         "",
					"repoHash":                        "",
					"repoHashDurationSeconds":         float64(0),
					"cacheHit":                        false,
					"cacheHitDurationSeconds":         float64(0),
					"buildDurationSeconds":            float64(0),
					"buildContextHash":                "",
					"initiator":                       "",
					"buildContextHashDurationSeconds": float64(0),
				},
			},
		},
		{
			name:  "not success event",
			input: &ImageBuildMetadata{},
			expectedEvent: &mockEvent{
				event:   "imageBuild",
				success: false,
				props: map[string]interface{}{
					"name":                            "",
					"repoURL":                         "",
					"repoHash":                        "",
					"repoHashDurationSeconds":         float64(0),
					"cacheHit":                        false,
					"cacheHitDurationSeconds":         float64(0),
					"buildDurationSeconds":            float64(0),
					"buildContextHash":                "",
					"initiator":                       "",
					"buildContextHashDurationSeconds": float64(0),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventMeta := &mockEvent{}
			tracker := &Tracker{
				trackFn: func(event string, success bool, props map[string]interface{}) {
					eventMeta = &mockEvent{
						event:   event,
						success: success,
						props:   props,
					}
				},
			}
			tracker.TrackImageBuild(context.Background(), tt.input)

			require.Equal(t, tt.expectedEvent.event, eventMeta.event)
			require.Equal(t, tt.expectedEvent.success, eventMeta.success)
			require.Equal(t, tt.expectedEvent.props, eventMeta.props)
		})
	}
}

func Test_ImageBuildMetadata_toProps(t *testing.T) {
	m := &ImageBuildMetadata{
		Name:                     "test-service",
		RepoURL:                  "http://myrepo.url",
		RepoHash:                 "repoHAsh",
		RepoHashDuration:         5 * time.Second,
		CacheHit:                 true,
		CacheHitDuration:         5 * time.Second,
		BuildDuration:            5 * time.Second,
		BuildContextHash:         "contextHash",
		BuildContextHashDuration: 5 * time.Second,
		Initiator:                "me",
	}

	expectedProps := map[string]interface{}{
		"name":                            "665653223b1e8bfa2d462b3adb06d49f8984052e5df03d7fd2365293a102fce8",
		"repoURL":                         "82eec095b1cc767833c5e4b5d7b02a6df10c0f284127c7e840e1f460b1896067",
		"repoHash":                        "repoHAsh",
		"repoHashDurationSeconds":         float64(5),
		"cacheHit":                        true,
		"cacheHitDurationSeconds":         float64(5),
		"buildDurationSeconds":            float64(5),
		"buildContextHash":                "contextHash",
		"buildContextHashDurationSeconds": float64(5),
		"initiator":                       "me",
	}

	require.Equal(t, expectedProps, m.toProps())
}

func Test_NewImageBuildMetadata(t *testing.T) {
	require.Empty(t, NewImageBuildMetadata())
	require.IsType(t, &ImageBuildMetadata{}, NewImageBuildMetadata())
}
