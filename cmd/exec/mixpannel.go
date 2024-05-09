package exec

import "github.com/okteto/okteto/pkg/analytics"

// mixpannelTrack represents a track to be sent to mixpannel
type mixpannelTrack struct {
	metadata *analytics.TrackExecMetadata

	trackFunc func(m *analytics.TrackExecMetadata)
}

// SetMetadata sets the metadata for the track
func (t *mixpannelTrack) SetMetadata(metadata *analytics.TrackExecMetadata) {
	t.metadata = metadata
}

// TrackExec sends the track to mixpannel
func (t *mixpannelTrack) Track() {
	if t.metadata == nil {
		return
	}
	t.trackFunc(t.metadata)
}
