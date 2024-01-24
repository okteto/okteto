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

	"github.com/stretchr/testify/assert"
)

func TestDestroyTracker(t *testing.T) {
	tt := []struct {
		name     string
		expected mockEvent
		metadata DestroyMetadata
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
			tracker := Tracker{
				trackFn: func(event string, success bool, props map[string]any) {
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
