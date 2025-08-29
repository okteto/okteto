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

package stream

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCalculateExponentialBackoff(t *testing.T) {
	tests := []struct {
		name     string
		attempt  int
		expected time.Duration
	}{
		{
			name:     "attempt 1 - minimum delay enforced",
			attempt:  1,
			expected: 500 * time.Millisecond, // (2^1-1)*0.5 = 0.5s, but min capped at 500ms
		},
		{
			name:     "attempt 2 - base calculation",
			attempt:  2,
			expected: 1500 * time.Millisecond, // (2^2-1)*0.5 = 1.5s
		},
		{
			name:     "attempt 3 - growing delay",
			attempt:  3,
			expected: 3500 * time.Millisecond, // (2^3-1)*0.5 = 3.5s
		},
		{
			name:     "attempt 4 - larger delay",
			attempt:  4,
			expected: 7500 * time.Millisecond, // (2^4-1)*0.5 = 7.5s
		},
		{
			name:     "attempt 5 - approaching maximum",
			attempt:  5,
			expected: 15500 * time.Millisecond, // (2^5-1)*0.5 = 15.5s
		},
		{
			name:     "attempt 6 - capped at maximum",
			attempt:  6,
			expected: 30 * time.Second, // (2^6-1)*0.5 = 31.5s, but max capped at 30s
		},
		{
			name:     "attempt 10 - still capped",
			attempt:  10,
			expected: 30 * time.Second, // Very large, but capped at 30s
		},
		{
			name:     "attempt 15 - still capped",
			attempt:  15,
			expected: 30 * time.Second, // Very large, but capped at 30s
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delay := calculateExponentialBackoff(tt.attempt)
			assert.Equal(t, tt.expected, delay,
				"Expected exact delay %v for attempt %d, but got %v", tt.expected, tt.attempt, delay)
		})
	}
}

func TestCalculateExponentialBackoff_Deterministic(t *testing.T) {
	// Verify that the function returns the same value for the same input (deterministic)
	attempt := 5
	expectedDelay := 15500 * time.Millisecond // (2^5-1)*0.5 = 15.5s

	// Call multiple times to ensure deterministic behavior
	for i := 0; i < 10; i++ {
		delay := calculateExponentialBackoff(attempt)
		assert.Equal(t, expectedDelay, delay,
			"Expected deterministic behavior: same input should always produce same output")
	}
}

func TestCalculateExponentialBackoff_MinimumDelay(t *testing.T) {
	// Test that very small attempts still respect minimum delay
	delay := calculateExponentialBackoff(1)
	assert.Equal(t, 500*time.Millisecond, delay, "Should enforce minimum delay of 500ms")

	// Also test attempt 0 (edge case)
	delay0 := calculateExponentialBackoff(0)
	assert.Equal(t, 500*time.Millisecond, delay0, "Should enforce minimum delay of 500ms for attempt 0")
}

func TestCalculateExponentialBackoff_MaximumDelay(t *testing.T) {
	// Test that large attempts are capped at 30 seconds
	largeAttempts := []int{6, 10, 15, 20, 100}
	for _, attempt := range largeAttempts {
		delay := calculateExponentialBackoff(attempt)
		assert.Equal(t, 30*time.Second, delay,
			"Delay should be exactly 30s for large attempt %d, got %v", attempt, delay)
	}
}
