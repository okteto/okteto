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
	"sync"
	"time"

	"github.com/okteto/okteto/pkg/analytics"
)

// ConnectorMetrics collects metrics for BuildKit connector analytics.
// It is designed to be embedded or composed into any connector type.
type ConnectorMetrics struct {
	sessionID         string
	connectorType     analytics.ConnectorType
	tracker           *analytics.Tracker
	mu                sync.Mutex
	StartTime         time.Time // Exported for use by connectors
	queueWaitDuration time.Duration
	maxQueuePosition  int
	lastQueueReason   string
	errReason         string
}

// NewConnectorMetrics creates a new ConnectorMetrics for the given connector type and session ID
func NewConnectorMetrics(connectorType analytics.ConnectorType, sessionID string) *ConnectorMetrics {
	return &ConnectorMetrics{
		sessionID:     sessionID,
		connectorType: connectorType,
		tracker:       analytics.NewAnalyticsTracker(),
	}
}

// StartTracking marks the start of a connection attempt
func (m *ConnectorMetrics) StartTracking() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.StartTime = time.Now()
	m.queueWaitDuration = 0
	m.maxQueuePosition = 0
	m.lastQueueReason = ""
}

// RecordQueueStatus updates queue metrics from a BuildKit API response
func (m *ConnectorMetrics) RecordQueueStatus(queuePosition int, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if queuePosition > 0 {
		if queuePosition > m.maxQueuePosition {
			m.maxQueuePosition = queuePosition
		}
		m.lastQueueReason = reason
	}
}

// SetErrReason sets the error reason for the connection failure
func (m *ConnectorMetrics) SetErrReason(reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errReason = reason
}

// TrackSuccess sends a success event to analytics
func (m *ConnectorMetrics) TrackSuccess() {
	m.track(true)
}

// TrackFailure sends a failure event to analytics
func (m *ConnectorMetrics) TrackFailure() {
	m.track(false)
}

func (m *ConnectorMetrics) track(success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.StartTime.IsZero() {
		m.queueWaitDuration = time.Since(m.StartTime)
	}

	metadata := &analytics.BuildkitConnectorMetadata{
		SessionID:         m.sessionID,
		ConnectorType:     m.connectorType,
		Success:           success,
		QueueWaitDuration: m.queueWaitDuration,
		MaxQueuePosition:  m.maxQueuePosition,
		QueueReason:       m.lastQueueReason,
		ErrReason:         m.errReason,
	}
	m.tracker.TrackBuildkitConnection(metadata)
}
