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

package build

import (
	"time"

	"github.com/moby/buildkit/client"
)

type contextSyncTracker struct {
	vertexes  map[string]bool
	startTime *time.Time
	endTime   *time.Time
	current   int64
	total     int64
}

func newContextSyncTracker() *contextSyncTracker {
	return &contextSyncTracker{
		vertexes: map[string]bool{},
	}
}

func (t *contextSyncTracker) Update(ss *client.SolveStatus) {
	for _, rawVertex := range ss.Vertexes {
		if isTransferringContext(rawVertex.Name) {
			t.vertexes[rawVertex.Digest.Encoded()] = true
			if rawVertex.Started != nil && t.startTime == nil {
				t.startTime = rawVertex.Started
			}
			if rawVertex.Completed != nil {
				t.endTime = rawVertex.Completed
			}
		}
	}

	for _, status := range ss.Statuses {
		if !t.vertexes[status.Vertex.Encoded()] {
			continue
		}
		if status.Current > t.current {
			t.current = status.Current
		}
		if status.Total > t.total {
			t.total = status.Total
		}
	}
}

func (t *contextSyncTracker) Current() int64 {
	return t.current
}

func (t *contextSyncTracker) Total() int64 {
	return t.total
}

func (t *contextSyncTracker) SyncedSize() int64 {
	if t.total > 0 {
		return t.total
	}
	return t.current
}

func (t *contextSyncTracker) Duration() time.Duration {
	if t.startTime == nil || t.endTime == nil {
		return 0
	}
	return t.endTime.Sub(*t.startTime)
}
