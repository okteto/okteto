package analytics

import (
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
)

type mockEvent struct {
	event   string
	success bool
	props   map[string]any
}

func TestDeployTracker(t *testing.T) {
	tt := []struct {
		name     string
		metadata DeployMetadata
		expected mockEvent
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
			tracker := AnalyticsTracker{
				TrackFn: func(event string, success bool, props map[string]any) {
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

func TestDestroyTracker(t *testing.T) {
	tt := []struct {
		name     string
		metadata DestroyMetadata
		expected mockEvent
	}{
		{
			name: "success destroy",
			metadata: DestroyMetadata{
				Success: true,
			},
			expected: mockEvent{
				event:   destroyEvent,
				success: true,
				props: map[string]any{
					"isDestroyAll": false,
					"isRemote":     false,
				},
			},
		},
		{
			name: "success destroy on remote",
			metadata: DestroyMetadata{
				Success:  true,
				IsRemote: true,
			},
			expected: mockEvent{
				event:   destroyEvent,
				success: true,
				props: map[string]any{
					"isDestroyAll": false,
					"isRemote":     true,
				},
			},
		},
		{
			name: "fail destroy all",
			metadata: DestroyMetadata{
				Success:      false,
				IsRemote:     false,
				IsDestroyAll: true,
			},
			expected: mockEvent{
				event:   destroyEvent,
				success: false,
				props: map[string]any{
					"isDestroyAll": true,
					"isRemote":     false,
				},
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			eventReceived := &mockEvent{}
			tracker := AnalyticsTracker{
				TrackFn: func(event string, success bool, props map[string]any) {
					eventReceived = &mockEvent{
						event:   event,
						success: success,
						props:   props,
					}
				},
			}

			tracker.TrackDestroy(tc.metadata)

			assert.Equal(t, tc.expected.event, eventReceived.event)
			assert.Equal(t, tc.expected.success, eventReceived.success)
			assert.Equal(t, tc.expected.props, eventReceived.props)
		})
	}
}
