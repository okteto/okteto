// Copyright 2026 The Okteto Authors
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
	"testing"
	"time"

	"github.com/moby/buildkit/client"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/require"
)

func TestContextSyncTrackerSyncedSize(t *testing.T) {
	tracker := newContextSyncTracker()
	contextDigest := digest.FromString("context")
	otherDigest := digest.FromString("other")

	tracker.Update(&client.SolveStatus{
		Vertexes: []*client.Vertex{
			{Digest: contextDigest, Name: "[internal] load build context"},
			{Digest: otherDigest, Name: "regular step"},
		},
	})

	tracker.Update(&client.SolveStatus{
		Statuses: []*client.VertexStatus{
			{Vertex: contextDigest, Current: 128, Total: 256},
			{Vertex: otherDigest, Current: 999, Total: 999},
		},
	})

	require.Equal(t, int64(128), tracker.Current())
	require.Equal(t, int64(256), tracker.Total())
	require.Equal(t, int64(256), tracker.SyncedSize())
}

func TestContextSyncTrackerDuration(t *testing.T) {
	tracker := newContextSyncTracker()
	contextDigest := digest.FromString("context")

	start := time.Now()
	end := start.Add(3 * time.Second)

	tracker.Update(&client.SolveStatus{
		Vertexes: []*client.Vertex{
			{Digest: contextDigest, Name: "[internal] load build context", Started: &start, Completed: &end},
		},
	})

	require.Equal(t, 3*time.Second, tracker.Duration())
}

func TestContextSyncTrackerDurationZeroWhenIncomplete(t *testing.T) {
	tracker := newContextSyncTracker()
	contextDigest := digest.FromString("context")

	start := time.Now()
	tracker.Update(&client.SolveStatus{
		Vertexes: []*client.Vertex{
			{Digest: contextDigest, Name: "[internal] load build context", Started: &start},
		},
	})

	require.Equal(t, time.Duration(0), tracker.Duration())
}

func TestContextSyncTrackerFallsBackToCurrentWhenTotalIsUnknown(t *testing.T) {
	tracker := newContextSyncTracker()
	contextDigest := digest.FromString("context")

	tracker.Update(&client.SolveStatus{
		Vertexes: []*client.Vertex{
			{Digest: contextDigest, Name: "[internal] load build context"},
		},
	})

	tracker.Update(&client.SolveStatus{
		Statuses: []*client.VertexStatus{
			{Vertex: contextDigest, Current: 512},
		},
	})

	require.Equal(t, int64(512), tracker.SyncedSize())
}
