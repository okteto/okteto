// Copyright 2023 The Okteto Authors
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

package time

import "time"

// Timer is the time.Timer wrapper to be able to mock it
type Timer struct {
	C     <-chan time.Time
	timer *time.Timer
}

// NewTimer creates a wrrapper of the time.Timer struct
func NewTimer(d time.Duration) *Timer {
	t := time.NewTimer(d)
	return &Timer{C: t.C, timer: t}
}

// Tick returns the channel that will be ticking
func (t *Timer) Tick() <-chan time.Time {
	return t.C
}
