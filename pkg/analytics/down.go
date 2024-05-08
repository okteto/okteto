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

// TrackDown sends a tracking event to mixpanel when the user deactivates a development container
func (a *Tracker) TrackDown(success bool) {
	a.trackFn(downEvent, success, nil)
}

// TrackDownVolumes sends a tracking event to mixpanel when the user deactivates a development container and its volumes
func (a *Tracker) TrackDownVolumes(success bool) {
	a.trackFn(downVolumesEvent, success, nil)
}
