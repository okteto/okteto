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

type TickerWithTimeoutInterface interface {
	TickerTick() <-chan time.Time
	TimeoutTick() <-chan time.Time
}

// TickerWithTimeout is the time.Ticker and timeout wrapper to be able to mock it
type TickerWithTimeout struct {
	Ticker  *Ticker
	Timeout *Timer
}

// NewTicker creates a wrrapper of the time.Ticker struct
func NewTickerWithTimeout(ticker, timeout time.Duration) TickerWithTimeoutInterface {
	t := NewTicker(ticker)
	timer := NewTimer(timeout)
	return &TickerWithTimeout{
		Ticker:  t,
		Timeout: timer,
	}
}

func (t *TickerWithTimeout) TickerTick() <-chan time.Time  { return t.Ticker.C }
func (t *TickerWithTimeout) TimeoutTick() <-chan time.Time { return t.Timeout.C }
