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
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
)

func TestDeployTracker(t *testing.T) {
	tt := []struct {
		expected mockEvent
		name     string
		metadata DeployMetadata
	}{
		{
			name: "all fields set",
			metadata: DeployMetadata{
				Success:                true,
				IsOktetoRepo:           true,
				Err:                    nil,
				Duration:               2 * time.Second,
				PipelineType:           model.PipelineType,
				DeployType:             "deploy",
				IsPreview:              true,
				HasDependenciesSection: true,
				HasBuildSection:        true,
				IsRemote:               true,
			},
			expected: mockEvent{
				event:   deployEvent,
				success: true,
				props: map[string]any{
					"pipelineType":           model.PipelineType,
					"isOktetoRepository":     true,
					"duration":               (2 * time.Second).Seconds(),
					"deployType":             "deploy",
					"isPreview":              true,
					"hasDependenciesSection": true,
					"hasBuildSection":        true,
					"isRemote":               true,
				},
			},
		},
		{
			name: "pipeline type not set",
			metadata: DeployMetadata{
				Success:                true,
				IsOktetoRepo:           true,
				Err:                    nil,
				Duration:               2 * time.Second,
				PipelineType:           "",
				DeployType:             "deploy",
				IsPreview:              true,
				HasDependenciesSection: true,
				HasBuildSection:        true,
				IsRemote:               true,
			},
			expected: mockEvent{
				event:   deployEvent,
				success: true,
				props: map[string]any{
					"pipelineType":           model.PipelineType,
					"isOktetoRepository":     true,
					"duration":               (2 * time.Second).Seconds(),
					"deployType":             "deploy",
					"isPreview":              true,
					"hasDependenciesSection": true,
					"hasBuildSection":        true,
					"isRemote":               true,
				},
			},
		},
		{
			name: "error set",
			metadata: DeployMetadata{
				Success:                true,
				IsOktetoRepo:           true,
				Err:                    assert.AnError,
				Duration:               2 * time.Second,
				PipelineType:           "",
				DeployType:             "deploy",
				IsPreview:              true,
				HasDependenciesSection: true,
				HasBuildSection:        true,
				IsRemote:               true,
			},
			expected: mockEvent{
				event:   deployEvent,
				success: true,
				props: map[string]any{
					"pipelineType":           model.PipelineType,
					"isOktetoRepository":     true,
					"duration":               (2 * time.Second).Seconds(),
					"deployType":             "deploy",
					"isPreview":              true,
					"hasDependenciesSection": true,
					"hasBuildSection":        true,
					"isRemote":               true,
					"error":                  assert.AnError.Error(),
				},
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			eventReceived := &mockEvent{}
			tracker := Tracker{
				trackFn: func(event string, success bool, props map[string]any) {
					eventReceived = &mockEvent{
						event:   event,
						success: success,
						props:   props,
					}
				},
			}

			tracker.TrackDeploy(tc.metadata)

			assert.Equal(t, tc.expected.event, eventReceived.event)
			assert.Equal(t, tc.expected.success, eventReceived.success)
			assert.Equal(t, tc.expected.props, eventReceived.props)
		})
	}
}

func TestAnalyticsTracker_TrackDeployStarted(t *testing.T) {
	var captured DeployStartedMetadata
	mock := &mockAnalyticsBackend{
		trackDeployStartedFn: func(meta DeployStartedMetadata) { captured = meta },
	}
	tracker := &Tracker{
		trackFn:  func(_ string, _ bool, _ map[string]any) {},
		backends: []analyticsBackend{mock},
	}

	meta := DeployStartedMetadata{Namespace: "dev-ns", RepoURL: "https://github.com/org/repo", IsRedeploy: true}
	tracker.TrackDeployStarted(meta)

	assert.Equal(t, meta, captured)
}

func TestDeployMetadata_ToPostHogProps(t *testing.T) {
	t.Run("new fields hashed and conditional", func(t *testing.T) {
		props := (&DeployMetadata{
			Success:           true,
			PipelineType:      model.StackType,
			RepoURL:           "https://github.com/org/repo",
			ManifestSyntax:    "compose",
			ParentExecutionID: "exec-1",
			IsPreview:         true,
			Duration:          90 * time.Second,
		}).toPostHogProps()

		assert.Equal(t, "compose", props["manifest_archetype"])
		assert.Equal(t, "compose", props["manifest_syntax"])
		assert.Equal(t, "preview", props["namespace_type"])
		assert.Equal(t, true, props["is_within_preview"])
		assert.Equal(t, "exec-1", props["parent_execution_id"])
		assert.Equal(t, 90.0, props["duration_seconds"])
		assert.Equal(t, hashString("https://github.com/org/repo"), props["repo_url"])
	})

	t.Run("empty optional fields omitted, defaults applied", func(t *testing.T) {
		props := (&DeployMetadata{Success: true}).toPostHogProps()

		assert.Equal(t, "pipeline", props["manifest_archetype"])
		assert.Equal(t, "regular", props["namespace_type"])
		assert.NotContains(t, props, "repo_url")
		assert.NotContains(t, props, "manifest_syntax")
		assert.NotContains(t, props, "parent_execution_id")
		assert.NotContains(t, props, "duration_seconds")
	})
}

func TestDeployStartedMetadata_ToPostHogProps(t *testing.T) {
	t.Run("repo_url hashed, parent_execution_id present", func(t *testing.T) {
		props := (&DeployStartedMetadata{
			RepoURL:           "https://github.com/org/repo",
			ParentExecutionID: "exec-1",
			IsPreview:         true,
			IsRedeploy:        true,
		}).toPostHogProps()

		assert.Equal(t, true, props["is_within_preview"])
		assert.Equal(t, true, props["is_redeploy"])
		assert.Equal(t, "exec-1", props["parent_execution_id"])
		assert.Equal(t, hashString("https://github.com/org/repo"), props["repo_url"])
	})

	t.Run("empty fields omitted", func(t *testing.T) {
		props := (&DeployStartedMetadata{}).toPostHogProps()

		assert.NotContains(t, props, "repo_url")
		assert.NotContains(t, props, "parent_execution_id")
		assert.Equal(t, false, props["is_within_preview"])
		assert.Equal(t, false, props["is_redeploy"])
	})
}
