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

package preview

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// Mock stream function for testing retry behavior
type mockStreamFunc struct {
	callCount    int32
	failCount    int32
	err          error
	callDuration time.Duration
	callTimes    []time.Time
}

func (m *mockStreamFunc) call(ctx context.Context) error {
	atomic.AddInt32(&m.callCount, 1)
	m.callTimes = append(m.callTimes, time.Now())
	
	// Simulate some processing time if specified
	if m.callDuration > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(m.callDuration):
		}
	}
	
	// Fail for the first N attempts, then succeed
	if m.failCount > 0 && atomic.LoadInt32(&m.callCount) <= m.failCount {
		return m.err
	}
	
	return nil
}

func (m *mockStreamFunc) getCallCount() int {
	return int(atomic.LoadInt32(&m.callCount))
}

func TestStreamWithExponentialBackoff_Success(t *testing.T) {
	ctx := context.Background()
	timeout := 10 * time.Second
	
	mock := &mockStreamFunc{}
	
	err := streamWithExponentialBackoff(ctx, timeout, mock.call, "test operation")
	
	assert.NoError(t, err)
	assert.Equal(t, 1, mock.getCallCount(), "Should succeed on first attempt")
}

func TestStreamWithExponentialBackoff_SuccessAfterRetries(t *testing.T) {
	ctx := context.Background()
	timeout := 30 * time.Second
	
	mock := &mockStreamFunc{
		failCount: 3, // Fail first 3 attempts, succeed on 4th
		err:       errors.New("temporary failure"),
	}
	
	start := time.Now()
	err := streamWithExponentialBackoff(ctx, timeout, mock.call, "test operation")
	duration := time.Since(start)
	
	assert.NoError(t, err)
	assert.Equal(t, 4, mock.getCallCount(), "Should succeed on 4th attempt")
	
	// Verify we actually waited between retries (should be several seconds)
	assert.Greater(t, duration, 2*time.Second, "Should have waited for exponential backoff")
}

func TestStreamWithExponentialBackoff_TimeoutReached(t *testing.T) {
	ctx := context.Background()
	timeout := 2 * time.Second // Short timeout
	
	mock := &mockStreamFunc{
		failCount: 100, // Always fail
		err:       errors.New("persistent failure"),
	}
	
	start := time.Now()
	err := streamWithExponentialBackoff(ctx, timeout, mock.call, "test operation")
	duration := time.Since(start)
	
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	
	// Should have timed out around the expected timeout
	assert.GreaterOrEqual(t, duration, timeout)
	assert.Less(t, duration, timeout+500*time.Millisecond) // Some tolerance
	
	// Should have made multiple attempts within the timeout
	assert.GreaterOrEqual(t, mock.getCallCount(), 2, "Should have made multiple attempts")
}

func TestStreamWithExponentialBackoff_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	timeout := 30 * time.Second
	
	mock := &mockStreamFunc{
		failCount: 100, // Always fail
		err:       errors.New("persistent failure"),
	}
	
	// Cancel context after 1 second
	go func() {
		time.Sleep(1 * time.Second)
		cancel()
	}()
	
	start := time.Now()
	err := streamWithExponentialBackoff(ctx, timeout, mock.call, "test operation")
	duration := time.Since(start)
	
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	
	// Should have stopped quickly after cancellation
	assert.Less(t, duration, 2*time.Second)
	assert.GreaterOrEqual(t, mock.getCallCount(), 1, "Should have made at least one attempt")
}

func TestStreamWithExponentialBackoff_ErrorPropagation(t *testing.T) {
	ctx := context.Background()
	timeout := 1 * time.Second
	
	testError := errors.New("specific test error")
	mock := &mockStreamFunc{
		failCount: 100, // Always fail
		err:       testError,
	}
	
	err := streamWithExponentialBackoff(ctx, timeout, mock.call, "test operation")
	
	// Should timeout, not return the specific error (since we keep retrying)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestStreamWithExponentialBackoff_ImmediateContextErrors(t *testing.T) {
	timeout := 10 * time.Second
	
	tests := []struct {
		name    string
		ctx     context.Context
		wantErr error
	}{
		{
			name:    "canceled context",
			ctx:     func() context.Context { ctx, cancel := context.WithCancel(context.Background()); cancel(); return ctx }(),
			wantErr: context.Canceled,
		},
		{
			name:    "expired context",
			ctx:     func() context.Context { ctx, cancel := context.WithTimeout(context.Background(), -1*time.Second); defer cancel(); return ctx }(),
			wantErr: context.DeadlineExceeded,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockStreamFunc{
				err: errors.New("should not be called"),
			}
			
			err := streamWithExponentialBackoff(tt.ctx, timeout, mock.call, "test operation")
			
			assert.Error(t, err)
			assert.ErrorIs(t, err, tt.wantErr)
			// Should not have made any calls due to immediate context error
			assert.Equal(t, 0, mock.getCallCount())
		})
	}
}

func TestStreamWithExponentialBackoff_SlowFunction(t *testing.T) {
	ctx := context.Background()
	timeout := 10 * time.Second
	
	mock := &mockStreamFunc{
		callDuration: 500 * time.Millisecond, // Each call takes 500ms
		failCount:    2,                      // Fail first 2 attempts
		err:          errors.New("temporary failure"),
	}
	
	start := time.Now()
	err := streamWithExponentialBackoff(ctx, timeout, mock.call, "test operation")
	duration := time.Since(start)
	
	assert.NoError(t, err)
	assert.Equal(t, 3, mock.getCallCount(), "Should succeed on 3rd attempt")
	
	// Should account for both retry delays and function call duration
	expectedMinDuration := 2*500*time.Millisecond + 500*time.Millisecond // 2 failed calls + backoff delay
	assert.GreaterOrEqual(t, duration, expectedMinDuration)
}

func TestStreamWithExponentialBackoff_RetryTiming(t *testing.T) {
	ctx := context.Background()
	timeout := 10 * time.Second
	
	mock := &mockStreamFunc{
		failCount: 3, // Fail first 3 attempts
		err:       errors.New("temporary failure"),
	}
	
	start := time.Now()
	err := streamWithExponentialBackoff(ctx, timeout, mock.call, "test operation")
	
	require.NoError(t, err)
	require.Equal(t, 4, mock.getCallCount(), "Should have made 4 attempts")
	require.Len(t, mock.callTimes, 4, "Should have recorded 4 call times")
	
	// Verify exact exponential backoff timing between calls (deterministic without jitter)
	// Call 1 -> Call 2: should wait exactly 500ms (attempt 1 backoff, min enforced)  
	// Call 2 -> Call 3: should wait exactly 1500ms (attempt 2 backoff)
	// Call 3 -> Call 4: should wait exactly 3500ms (attempt 3 backoff)
	
	delay1 := mock.callTimes[1].Sub(mock.callTimes[0])
	delay2 := mock.callTimes[2].Sub(mock.callTimes[1])  
	delay3 := mock.callTimes[3].Sub(mock.callTimes[2])
	
	// Allow small tolerance for timing precision (±50ms)
	tolerance := 50 * time.Millisecond
	
	expectedDelay1 := 500 * time.Millisecond  // attempt 1: min enforced
	expectedDelay2 := 1500 * time.Millisecond // attempt 2: (2^2-1)*0.5 = 1.5s
	expectedDelay3 := 3500 * time.Millisecond // attempt 3: (2^3-1)*0.5 = 3.5s
	
	assert.InDelta(t, expectedDelay1.Nanoseconds(), delay1.Nanoseconds(), float64(tolerance.Nanoseconds()), 
		"First retry delay should be %v (±%v), got %v", expectedDelay1, tolerance, delay1)
	assert.InDelta(t, expectedDelay2.Nanoseconds(), delay2.Nanoseconds(), float64(tolerance.Nanoseconds()), 
		"Second retry delay should be %v (±%v), got %v", expectedDelay2, tolerance, delay2)
	assert.InDelta(t, expectedDelay3.Nanoseconds(), delay3.Nanoseconds(), float64(tolerance.Nanoseconds()), 
		"Third retry delay should be %v (±%v), got %v", expectedDelay3, tolerance, delay3)
	
	// Verify total duration matches expected cumulative delays
	expectedTotalDelay := expectedDelay1 + expectedDelay2 + expectedDelay3
	totalDuration := time.Since(start)
	assert.GreaterOrEqual(t, totalDuration, expectedTotalDelay, 
		"Total duration should include all retry delays: %v", expectedTotalDelay)
}