// Copyright 2025 The Okteto Authors
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

	"github.com/stretchr/testify/assert"
)

func TestBuildkitConnectorMetadata_toProps(t *testing.T) {
	tests := []struct {
		name     string
		metadata *BuildkitConnectorMetadata
		expected map[string]interface{}
	}{
		{
			name: "portforward with queue",
			metadata: &BuildkitConnectorMetadata{
				SessionID:             "session-123",
				ConnectorType:         ConnectorTypePortForward,
				Success:               true,
				WasFallback:           false,
				WasQueued:             true,
				QueueWaitDuration:     30 * time.Second,
				MaxQueuePosition:      3,
				QueueReason:           "ALL_PODS_BUSY",
				ServiceReadyDuration:  2 * time.Second,
				PodReused:             false,
				WaitingForPodTimedOut: false,
			},
			expected: map[string]interface{}{
				"sessionId":                   "session-123",
				"connectorType":               "portforward",
				"wasFallback":                 false,
				"wasQueued":                   true,
				"queueWaitDurationSeconds":    float64(30),
				"maxQueuePosition":            3,
				"queueReason":                 "ALL_PODS_BUSY",
				"serviceReadyDurationSeconds": float64(2),
				"podReused":                   false,
				"waitingForPodTimedOut":       false,
			},
		},
		{
			name: "incluster with pod reuse",
			metadata: &BuildkitConnectorMetadata{
				SessionID:             "session-456",
				ConnectorType:         ConnectorTypeInCluster,
				Success:               true,
				WasFallback:           false,
				WasQueued:             false,
				QueueWaitDuration:     0,
				MaxQueuePosition:      0,
				QueueReason:           "",
				ServiceReadyDuration:  1 * time.Second,
				PodReused:             true,
				WaitingForPodTimedOut: false,
			},
			expected: map[string]interface{}{
				"sessionId":                   "session-456",
				"connectorType":               "incluster",
				"wasFallback":                 false,
				"wasQueued":                   false,
				"queueWaitDurationSeconds":    float64(0),
				"maxQueuePosition":            0,
				"queueReason":                 "",
				"serviceReadyDurationSeconds": float64(1),
				"podReused":                   true,
				"waitingForPodTimedOut":       false,
			},
		},
		{
			name: "ingress simple connection",
			metadata: &BuildkitConnectorMetadata{
				SessionID:             "session-789",
				ConnectorType:         ConnectorTypeIngress,
				Success:               true,
				WasFallback:           false,
				WasQueued:             false,
				QueueWaitDuration:     0,
				MaxQueuePosition:      0,
				QueueReason:           "",
				ServiceReadyDuration:  500 * time.Millisecond,
				PodReused:             false,
				WaitingForPodTimedOut: false,
			},
			expected: map[string]interface{}{
				"sessionId":                   "session-789",
				"connectorType":               "ingress",
				"wasFallback":                 false,
				"wasQueued":                   false,
				"queueWaitDurationSeconds":    float64(0),
				"maxQueuePosition":            0,
				"queueReason":                 "",
				"serviceReadyDurationSeconds": float64(0.5),
				"podReused":                   false,
				"waitingForPodTimedOut":       false,
			},
		},
		{
			name: "ingress fallback connection",
			metadata: &BuildkitConnectorMetadata{
				SessionID:             "session-fallback",
				ConnectorType:         ConnectorTypeIngress,
				Success:               true,
				WasFallback:           true,
				WasQueued:             false,
				QueueWaitDuration:     0,
				MaxQueuePosition:      0,
				QueueReason:           "",
				ServiceReadyDuration:  1 * time.Second,
				PodReused:             false,
				WaitingForPodTimedOut: false,
			},
			expected: map[string]interface{}{
				"sessionId":                   "session-fallback",
				"connectorType":               "ingress",
				"wasFallback":                 true,
				"wasQueued":                   false,
				"queueWaitDurationSeconds":    float64(0),
				"maxQueuePosition":            0,
				"queueReason":                 "",
				"serviceReadyDurationSeconds": float64(1),
				"podReused":                   false,
				"waitingForPodTimedOut":       false,
			},
		},
		{
			name: "timeout failure",
			metadata: &BuildkitConnectorMetadata{
				SessionID:             "session-timeout",
				ConnectorType:         ConnectorTypePortForward,
				Success:               false,
				WasFallback:           false,
				WasQueued:             true,
				QueueWaitDuration:     10 * time.Minute,
				MaxQueuePosition:      10,
				QueueReason:           "NO_PODS_AVAILABLE",
				ServiceReadyDuration:  0,
				PodReused:             false,
				WaitingForPodTimedOut: true,
			},
			expected: map[string]interface{}{
				"sessionId":                   "session-timeout",
				"connectorType":               "portforward",
				"wasFallback":                 false,
				"wasQueued":                   true,
				"queueWaitDurationSeconds":    float64(600),
				"maxQueuePosition":            10,
				"queueReason":                 "NO_PODS_AVAILABLE",
				"serviceReadyDurationSeconds": float64(0),
				"podReused":                   false,
				"waitingForPodTimedOut":       true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			props := tt.metadata.toProps()
			assert.Equal(t, tt.expected, props)
		})
	}
}


func TestTrackBuildkitConnection(t *testing.T) {
	var capturedEvent string
	var capturedSuccess bool
	var capturedProps map[string]interface{}

	tracker := &Tracker{
		trackFn: func(event string, success bool, props map[string]interface{}) {
			capturedEvent = event
			capturedSuccess = success
			capturedProps = props
		},
	}

	metadata := &BuildkitConnectorMetadata{
		SessionID:             "test-session-id",
		ConnectorType:         ConnectorTypePortForward,
		Success:               true,
		WasFallback:           false,
		WasQueued:             true,
		QueueWaitDuration:     5 * time.Second,
		MaxQueuePosition:      2,
		QueueReason:           "QUEUE_POSITION",
		ServiceReadyDuration:  1 * time.Second,
		PodReused:             false,
		WaitingForPodTimedOut: false,
	}

	tracker.TrackBuildkitConnection(metadata)

	assert.Equal(t, "BuildkitConnection", capturedEvent)
	assert.True(t, capturedSuccess)
	assert.Equal(t, "test-session-id", capturedProps["sessionId"])
	assert.Equal(t, "portforward", capturedProps["connectorType"])
	assert.Equal(t, false, capturedProps["wasFallback"])
	assert.Equal(t, true, capturedProps["wasQueued"])
	assert.Equal(t, float64(5), capturedProps["queueWaitDurationSeconds"])
	assert.Equal(t, 2, capturedProps["maxQueuePosition"])
	assert.Equal(t, "QUEUE_POSITION", capturedProps["queueReason"])
}
