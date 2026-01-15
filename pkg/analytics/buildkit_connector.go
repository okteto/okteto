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

import "time"

const buildkitConnectionEvent = "BuildkitConnection"

// ConnectorType identifies the type of BuildKit connector used
type ConnectorType string

const (
	ConnectorTypePortForward ConnectorType = "portforward"
	ConnectorTypeInCluster   ConnectorType = "incluster"
)

// BuildkitConnectorMetadata contains the metadata for BuildKit connector analytics
type BuildkitConnectorMetadata struct {
	// SessionID is a unique identifier for this connection session (for filtering in Mixpanel)
	SessionID string

	// ConnectorType identifies which connector was used (ingress, portforward, incluster)
	ConnectorType ConnectorType

	// Success indicates if the connection was established successfully
	Success bool

	// --- Queue Metrics (for portforward and incluster) ---

	// QueueWaitDuration is the total time spent in assigning the buildkit pod until getting a pod
	QueueWaitDuration time.Duration

	// MaxQueuePosition is the highest queue position observed
	MaxQueuePosition int

	// QueueReason is the last wait reason observed (if queued)
	// Values: QUEUE_POSITION, NO_PODS_AVAILABLE, ALL_PODS_BUSY, PODS_SCALING
	QueueReason string

	// --- Error Tracking ---

	// ErrReason is the reason for the connection failure
	// Values: QueueTimeout, PortForwardCreation, ConnectionLost, BackendInternalError, IncompatibleBackend, etc.
	ErrReason string
}

func (m *BuildkitConnectorMetadata) toProps() map[string]interface{} {
	return map[string]interface{}{
		"sessionId":                m.SessionID,
		"connectorType":            string(m.ConnectorType),
		"queueWaitDurationSeconds": m.QueueWaitDuration.Seconds(),
		"maxQueuePosition":         m.MaxQueuePosition,
		"queueReason":              m.QueueReason,
		"errReason":                m.ErrReason,
	}
}

// TrackBuildkitConnection sends a tracking event to mixpanel with BuildKit connector metrics
func (a *Tracker) TrackBuildkitConnection(m *BuildkitConnectorMetadata) {
	a.trackFn(buildkitConnectionEvent, m.Success, m.toProps())
}
