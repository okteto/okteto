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

package analytics

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTrackTest(t *testing.T) {
	var event string
	var success bool
	var wasBuilt bool
	var wasDeployed bool
	var duration float64
	var stagesCount int
	var errStr string
	tracker := Tracker{
		trackFn: func(ev string, ok bool, props map[string]any) {
			event = ev
			success = ok
			wasBuilt = props["wasBuilt"].(bool)
			wasDeployed = props["wasDeployed"].(bool)
			duration = props["duration"].(float64)
			stagesCount = props["stagesCount"].(int)
			errStr = props["error"].(string)
		},
	}

	tracker.TrackTest(TestMetadata{
		WasDeployed: true,
		WasBuilt:    true,
		Success:     false,
		Duration:    time.Second * 5,
		StagesCount: 3,
		Err:         fmt.Errorf("my-error"),
	})

	require.True(t, wasDeployed)
	require.True(t, wasBuilt)
	require.False(t, success)
	require.Equal(t, errStr, "my-error")
	require.Equal(t, "Test", event)
	require.Equal(t, float64(5), duration)
	require.Equal(t, 3, stagesCount)
}
