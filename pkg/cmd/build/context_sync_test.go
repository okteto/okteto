package build

import (
	"testing"

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
