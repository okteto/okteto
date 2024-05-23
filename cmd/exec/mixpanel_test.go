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

import (
	"testing"

	"github.com/okteto/okteto/pkg/analytics"
	"github.com/stretchr/testify/assert"
)

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

	track.Track(metadata)
	assert.Equal(t, true, mockedTrack.called)
}
