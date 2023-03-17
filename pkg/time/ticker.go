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

type TickerInterface interface {
	Tick() <-chan time.Time
}

// Ticker is the time.Ticker wrapper to be able to mock it
type Ticker struct {
	C      <-chan time.Time
	ticker *time.Ticker
}

// NewTicker creates a wrrapper of the time.Ticker struct
func NewTicker(d time.Duration) *Ticker {
	t := time.NewTicker(d)
	return &Ticker{C: t.C, ticker: t}
}

// Tick returns the channel that will be ticking
func (t *Ticker) Tick() <-chan time.Time {
	return t.C
}
