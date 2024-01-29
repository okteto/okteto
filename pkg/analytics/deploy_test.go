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
