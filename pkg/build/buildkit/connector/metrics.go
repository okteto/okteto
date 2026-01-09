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
	sessionID             string
	connectorType         analytics.ConnectorType
	tracker               *analytics.Tracker
	mu                    sync.Mutex
	StartTime             time.Time // Exported for use by connectors
	wasFallback           bool
	wasQueued             bool
	queueWaitDuration     time.Duration
	maxQueuePosition      int
	lastQueueReason       string
	podReused             bool
	waitingForPodTimedOut bool
	serviceReadyDuration  time.Duration
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
	m.wasQueued = false
	m.queueWaitDuration = 0
	m.maxQueuePosition = 0
	m.lastQueueReason = ""
	m.waitingForPodTimedOut = false
}

// RecordQueueStatus updates queue metrics from a BuildKit API response
func (m *ConnectorMetrics) RecordQueueStatus(queuePosition int, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if queuePosition > 0 {
		m.wasQueued = true
		if queuePosition > m.maxQueuePosition {
			m.maxQueuePosition = queuePosition
		}
		m.lastQueueReason = reason
	}
}

// SetPodReused marks whether an existing pod was reused
func (m *ConnectorMetrics) SetPodReused(reused bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.podReused = reused
}

// SetTimedOut marks the connection as timed out
func (m *ConnectorMetrics) SetWaitingForPodTimedOut(timedOut bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.waitingForPodTimedOut = timedOut
}

// SetWasFallback marks whether this connector was used as a fallback
func (m *ConnectorMetrics) SetWasFallback(wasFallback bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.wasFallback = wasFallback
}

// SetServiceReadyDuration sets the time waited for the service to be ready
func (m *ConnectorMetrics) SetServiceReadyDuration(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.serviceReadyDuration = d
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

	m.queueWaitDuration = time.Since(m.StartTime)

	metadata := &analytics.BuildkitConnectorMetadata{
		SessionID:             m.sessionID,
		ConnectorType:         m.connectorType,
		Success:               success,
		WasFallback:           m.wasFallback,
		WasQueued:             m.wasQueued,
		QueueWaitDuration:     m.queueWaitDuration,
		MaxQueuePosition:      m.maxQueuePosition,
		QueueReason:           m.lastQueueReason,
		ServiceReadyDuration:  m.serviceReadyDuration,
		PodReused:             m.podReused,
		WaitingForPodTimedOut: m.waitingForPodTimedOut,
	}
	m.tracker.TrackBuildkitConnection(metadata)
}
