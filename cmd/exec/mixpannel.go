// Copyright 2024 The Okteto Authors
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