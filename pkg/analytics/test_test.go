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
	var deployed bool
	var duration float64
	var stagesCount int
	var errStr string
	var commandOverride bool
	var profileSelected bool
	tracker := Tracker{
		trackFn: func(ev string, ok bool, props map[string]any) {
			event = ev
			success = ok
			wasBuilt = props["wasBuilt"].(bool)
			deployed = props["deployed"].(bool)
			duration = props["duration"].(float64)
			stagesCount = props["stagesCount"].(int)
			errStr = props["error"].(string)
			commandOverride = props["commandOverride"].(bool)
			profileSelected = props["profileSelected"].(bool)
		},
	}

	tracker.TrackTest(TestMetadata{
		ProfileSelected: true,
		Deployed:        true,
		WasBuilt:        true,
		Success:         false,
		Duration:        time.Second * 5,
		StagesCount:     3,
		Err:             fmt.Errorf("my-error"),
		CommandOverride: true,
	})

	require.True(t, deployed)
	require.True(t, wasBuilt)
	require.True(t, commandOverride)
	require.True(t, profileSelected)
	require.False(t, success)
	require.Equal(t, errStr, "my-error")
	require.Equal(t, "Test", event)
	require.Equal(t, float64(5), duration)
	require.Equal(t, 3, stagesCount)
}

func TestTrackTestWithoutCommandOverride(t *testing.T) {
	var commandOverride bool
	tracker := Tracker{
		trackFn: func(_ string, _ bool, props map[string]any) {
			commandOverride = props["commandOverride"].(bool)
		},
	}

	tracker.TrackTest(TestMetadata{Success: true})

	require.False(t, commandOverride)
}

func TestTrackTestWithoutProfileSelected(t *testing.T) {
	var profileSelected bool
	tracker := Tracker{
		trackFn: func(_ string, _ bool, props map[string]any) {
			profileSelected = props["profileSelected"].(bool)
		},
	}

	tracker.TrackTest(TestMetadata{Success: true})

	require.False(t, profileSelected)
}
