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
	"time"
)

const testEvent = "Test"

// TestMetadata contains the metadata of a test command execution event
type TestMetadata struct {
	// Err is the error (if any) that occurred during test execution. Note that
	// this is NOT an error in the test but rather a error encounterd in the setup
	Err error

	// Duration is the total duration of the test execution. Note that this includes
	// potential build and deploy times (not just the tests)
	Duration time.Duration

	// WasDeployed is whether a deploy was needed as part of the test
	WasDeployed bool

	// WasBuilt is whether at least one image had to be built to run the tests
	WasBuilt bool

	// Success is whether the tests succeeded or not
	Success bool

	// StagesCount is the number of sections that were tested (entries in the manifest test map)
	StagesCount int
}

// TrackTest sends a tracking event to mixpanel when the user runs test through the command okteto test
func (a *Tracker) TrackTest(metadata TestMetadata) {
	props := map[string]any{
		"duration":    metadata.Duration.Seconds(),
		"wasDeployed": metadata.WasDeployed,
		"wasBuilt":    metadata.WasBuilt,
		"success":     metadata.Success,
		"stagesCount": metadata.StagesCount,
	}
	if metadata.Err != nil {
		props["error"] = metadata.Err.Error()
	}
	a.trackFn(testEvent, metadata.Success, props)
}
