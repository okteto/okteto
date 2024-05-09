package exec

import (
	"testing"

	"github.com/okteto/okteto/pkg/analytics"
	"github.com/stretchr/testify/assert"
)

func TestMixpannelTrack_SetMetadata(t *testing.T) {
	track := &mixpannelTrack{}
	metadata := &analytics.TrackExecMetadata{
		Mode:               "test",
		FirstArgIsDev:      true,
		Success:            true,
		IsOktetoRepository: true,
	}

	track.SetMetadata(metadata)

	assert.Equal(t, metadata, track.metadata)
}

type mockTrack struct {
	called bool
}

func (mt *mockTrack) mockTrackFunc(metadata *analytics.TrackExecMetadata) {
	mt.called = true
}

func TestMixpannelTrack_TrackExec(t *testing.T) {
	mockedTrack := &mockTrack{}
	track := &mixpannelTrack{
		trackFunc: mockedTrack.mockTrackFunc,
	}

	metadata := &analytics.TrackExecMetadata{
		Mode:               "test",
		FirstArgIsDev:      true,
		Success:            true,
		IsOktetoRepository: true,
	}

	track.metadata = nil
	track.Track()
	assert.Equal(t, false, mockedTrack.called)

	track.metadata = metadata
	track.Track()
	assert.Equal(t, true, mockedTrack.called)
}
