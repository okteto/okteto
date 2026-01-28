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
				SessionID:         "session-123",
				ConnectorType:     ConnectorTypePortForward,
				Success:           true,
				QueueWaitDuration: 30 * time.Second,
				MaxQueuePosition:  3,
				QueueReason:       "ALL_PODS_BUSY",
				ErrReason:         "",
			},
			expected: map[string]interface{}{
				"sessionId":                "session-123",
				"connectorType":            "portforward",
				"queueWaitDurationSeconds": float64(30),
				"maxQueuePosition":         3,
				"queueReason":              "ALL_PODS_BUSY",
				"errReason":                "",
			},
		},
		{
			name: "incluster connection",
			metadata: &BuildkitConnectorMetadata{
				SessionID:         "session-456",
				ConnectorType:     ConnectorTypeInCluster,
				Success:           true,
				QueueWaitDuration: 0,
				MaxQueuePosition:  0,
				QueueReason:       "",
				ErrReason:         "",
			},
			expected: map[string]interface{}{
				"sessionId":                "session-456",
				"connectorType":            "incluster",
				"queueWaitDurationSeconds": float64(0),
				"maxQueuePosition":         0,
				"queueReason":              "",
				"errReason":                "",
			},
		},
		{
			name: "queue timeout failure",
			metadata: &BuildkitConnectorMetadata{
				SessionID:         "session-timeout",
				ConnectorType:     ConnectorTypePortForward,
				Success:           false,
				QueueWaitDuration: 10 * time.Minute,
				MaxQueuePosition:  10,
				QueueReason:       "NO_PODS_AVAILABLE",
				ErrReason:         "QueueTimeout",
			},
			expected: map[string]interface{}{
				"sessionId":                "session-timeout",
				"connectorType":            "portforward",
				"queueWaitDurationSeconds": float64(600),
				"maxQueuePosition":         10,
				"queueReason":              "NO_PODS_AVAILABLE",
				"errReason":                "QueueTimeout",
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
		SessionID:         "test-session-id",
		ConnectorType:     ConnectorTypePortForward,
		Success:           true,
		QueueWaitDuration: 5 * time.Second,
		MaxQueuePosition:  2,
		QueueReason:       "QUEUE_POSITION",
		ErrReason:         "",
	}

	tracker.TrackBuildkitConnection(metadata)

	assert.Equal(t, "BuildkitConnection", capturedEvent)
	assert.True(t, capturedSuccess)
	assert.Equal(t, "test-session-id", capturedProps["sessionId"])
	assert.Equal(t, "portforward", capturedProps["connectorType"])
	assert.Equal(t, float64(5), capturedProps["queueWaitDurationSeconds"])
	assert.Equal(t, 2, capturedProps["maxQueuePosition"])
	assert.Equal(t, "QUEUE_POSITION", capturedProps["queueReason"])
	assert.Equal(t, "", capturedProps["errReason"])
}
