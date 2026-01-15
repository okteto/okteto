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

package connector

import (
	"testing"

	"github.com/okteto/okteto/pkg/analytics"
	"github.com/stretchr/testify/assert"
)

func TestNewConnectorMetrics(t *testing.T) {
	tests := []struct {
		name          string
		connectorType analytics.ConnectorType
		sessionID     string
	}{
		{
			name:          "portforward",
			connectorType: analytics.ConnectorTypePortForward,
			sessionID:     "session-pf-123",
		},
		{
			name:          "incluster",
			connectorType: analytics.ConnectorTypeInCluster,
			sessionID:     "session-ic-456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewConnectorMetrics(tt.connectorType, tt.sessionID)
			assert.NotNil(t, m)
			assert.Equal(t, tt.connectorType, m.connectorType)
			assert.Equal(t, tt.sessionID, m.sessionID)
			assert.NotNil(t, m.tracker)
		})
	}
}

func TestConnectorMetrics_StartTracking(t *testing.T) {
	m := NewConnectorMetrics(analytics.ConnectorTypePortForward, "test-session")

	// Set some values first
	m.maxQueuePosition = 5

	// Start tracking should reset all values
	m.StartTracking()

	assert.Equal(t, 0, m.maxQueuePosition)
	assert.Equal(t, "", m.lastQueueReason)
	assert.False(t, m.StartTime.IsZero())
}

func TestConnectorMetrics_RecordQueueStatus(t *testing.T) {
	tests := []struct {
		name                     string
		queuePosition            int
		reason                   string
		expectedMaxQueuePosition int
		expectedReason           string
	}{
		{
			name:                     "no queue",
			queuePosition:            0,
			reason:                   "",
			expectedMaxQueuePosition: 0,
			expectedReason:           "",
		},
		{
			name:                     "in queue",
			queuePosition:            3,
			reason:                   "ALL_PODS_BUSY",
			expectedMaxQueuePosition: 3,
			expectedReason:           "ALL_PODS_BUSY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewConnectorMetrics(analytics.ConnectorTypePortForward, "test-session")
			m.RecordQueueStatus(tt.queuePosition, tt.reason)

			assert.Equal(t, tt.expectedMaxQueuePosition, m.maxQueuePosition)
			assert.Equal(t, tt.expectedReason, m.lastQueueReason)
		})
	}
}

func TestConnectorMetrics_RecordQueueStatus_MaxValues(t *testing.T) {
	m := NewConnectorMetrics(analytics.ConnectorTypePortForward, "test-session")

	// First call with position 3
	m.RecordQueueStatus(3, "QUEUE_POSITION")
	assert.Equal(t, 3, m.maxQueuePosition)

	// Second call with position 5 - should update max value
	m.RecordQueueStatus(5, "ALL_PODS_BUSY")
	assert.Equal(t, 5, m.maxQueuePosition)
	assert.Equal(t, "ALL_PODS_BUSY", m.lastQueueReason)

	// Third call with position 2 - should NOT update max value
	m.RecordQueueStatus(2, "PODS_SCALING")
	assert.Equal(t, 5, m.maxQueuePosition)
	assert.Equal(t, "PODS_SCALING", m.lastQueueReason) // reason always updates
}

func TestConnectorMetrics_SetErrReason(t *testing.T) {
	m := NewConnectorMetrics(analytics.ConnectorTypePortForward, "test-session")

	assert.Equal(t, "", m.errReason)
	m.SetErrReason("QueueTimeout")
	assert.Equal(t, "QueueTimeout", m.errReason)
	m.SetErrReason("PortForwardCreation")
	assert.Equal(t, "PortForwardCreation", m.errReason)
}
