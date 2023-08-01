package analytics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
