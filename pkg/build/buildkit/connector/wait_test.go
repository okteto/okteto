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

package connector

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moby/buildkit/client"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/stretchr/testify/assert"
)

// MockSleeper is a mock implementation of the sleeper interface.
type MockSleeper struct {
	Calls int
}

func (s *MockSleeper) Sleep(duration time.Duration) {
	s.Calls++
}

type fakeBuildkitClientFactoryWithRetries struct {
	err      error
	attempts int
}

func (f *fakeBuildkitClientFactoryWithRetries) GetBuildkitClient(ctx context.Context) (*client.Client, error) {
	f.attempts++
	return nil, f.err
}

// TestWaitUntilIsUp tests the WaitUntilIsUp method with various scenarios.
func TestWaitUntilIsUp(t *testing.T) {
	tests := []struct {
		expectedErr           error
		sleeper               *MockSleeper
		buildkitClientFactory *fakeBuildkitClientFactoryWithRetries
		name                  string
		maxWaitTime           time.Duration
		retryInterval         time.Duration
		expectedSleeperCalls  int
	}{
		{
			name:          "RetriesOnGetBuildkitClientError",
			maxWaitTime:   100 * time.Millisecond,
			retryInterval: 0,
			sleeper:       &MockSleeper{},
			buildkitClientFactory: &fakeBuildkitClientFactoryWithRetries{
				err: errors.New("connection refused"),
			},
			expectedErr:          errors.New("buildkit service not available"),
			expectedSleeperCalls: 1, // At least one sleep before timeout
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bw := &Waiter{
				logger:        io.NewIOController(),
				maxWaitTime:   tt.maxWaitTime,
				retryInterval: tt.retryInterval,
				sleeper:       tt.sleeper,
			}

			err := bw.WaitUntilIsUp(context.Background(), tt.buildkitClientFactory.GetBuildkitClient)
			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.ErrorContains(t, err, tt.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}

			// Verify that at least one retry happened
			assert.GreaterOrEqual(t, tt.sleeper.Calls, tt.expectedSleeperCalls)
			assert.GreaterOrEqual(t, tt.buildkitClientFactory.attempts, 1)
		})
	}
}

func TestNewBuildkitClientWaiter(t *testing.T) {
	tests := []struct {
		envVars             map[string]string
		name                string
		expectedMaxWaitTime time.Duration
		expectedRetryTime   time.Duration
	}{
		{
			name: "DefaultValues",
			envVars: map[string]string{
				maxBuildkitWaitTimeEnvVar:   "",
				retryBuildkitIntervalEnvVar: "",
			},
			expectedMaxWaitTime: maxWaitTime,
			expectedRetryTime:   retryTime,
		},
		{
			name: "CustomMaxWaitTime",
			envVars: map[string]string{
				maxBuildkitWaitTimeEnvVar:   "5m",
				retryBuildkitIntervalEnvVar: "",
			},
			expectedMaxWaitTime: 5 * time.Minute,
			expectedRetryTime:   retryTime,
		},
		{
			name: "CustomRetryTime",
			envVars: map[string]string{
				maxBuildkitWaitTimeEnvVar:   "",
				retryBuildkitIntervalEnvVar: "10s",
			},
			expectedMaxWaitTime: maxWaitTime,
			expectedRetryTime:   10 * time.Second,
		},
		{
			name: "CustomMaxWaitTimeAndRetryTime",
			envVars: map[string]string{
				maxBuildkitWaitTimeEnvVar:   "3m",
				retryBuildkitIntervalEnvVar: "15s",
			},
			expectedMaxWaitTime: 3 * time.Minute,
			expectedRetryTime:   15 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for key, value := range tt.envVars {
				if value != "" {
					t.Setenv(key, value)
				}
			}

			logger := io.NewIOController()
			bw := NewBuildkitClientWaiter(logger)

			assert.Equal(t, tt.expectedMaxWaitTime, bw.maxWaitTime)
			assert.Equal(t, tt.expectedRetryTime, bw.retryInterval)
			assert.IsType(t, &DefaultSleeper{}, bw.sleeper)
			assert.Equal(t, logger, bw.logger)
		})
	}
}
