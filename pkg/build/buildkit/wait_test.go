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

package buildkit

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
	err              error
	infoRetriever    clientInfoRetriever
	attempts         int
	successOnAttempt int
}

type fakeClientInfoRetriever struct {
	err []error
}

func (f *fakeClientInfoRetriever) Info(ctx context.Context) (*client.Info, error) {
	if len(f.err) > 0 {
		err := f.err[0]
		f.err = f.err[1:]
		return nil, err
	}
	return &client.Info{}, nil
}

func (f *fakeBuildkitClientFactoryWithRetries) GetBuildkitClient(ctx context.Context) (clientInfoRetriever, error) {
	f.attempts++
	if f.attempts >= f.successOnAttempt {
		return f.infoRetriever, nil
	}
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
		expectedAttempts      int
	}{
		{
			name:          "SuccessImmediately",
			maxWaitTime:   time.Minute,
			retryInterval: time.Second,
			sleeper:       &MockSleeper{},
			buildkitClientFactory: &fakeBuildkitClientFactoryWithRetries{
				successOnAttempt: 1,
				err:              nil,
				infoRetriever: &fakeClientInfoRetriever{
					err: nil,
				},
			},
			expectedErr:          nil,
			expectedSleeperCalls: 0,
			expectedAttempts:     1,
		},
		{
			name:          "RetriesGetBuildkitClient",
			maxWaitTime:   time.Minute,
			retryInterval: 0,
			sleeper:       &MockSleeper{},
			buildkitClientFactory: &fakeBuildkitClientFactoryWithRetries{
				successOnAttempt: 3,
				err:              errors.New("timeout"),
				infoRetriever:    &fakeClientInfoRetriever{},
			},
			expectedErr:          nil,
			expectedSleeperCalls: 2,
			expectedAttempts:     3,
		},
		{
			name:          "RetriesInfoTimeout",
			maxWaitTime:   time.Minute,
			retryInterval: 0,
			sleeper:       &MockSleeper{},
			buildkitClientFactory: &fakeBuildkitClientFactoryWithRetries{
				successOnAttempt: 1,
				err:              errors.New("timeout"),
				infoRetriever: &fakeClientInfoRetriever{
					err: []error{errors.New("timeout")},
				},
			},
			expectedErr:          nil,
			expectedSleeperCalls: 1,
			expectedAttempts:     2,
		},
		{
			name:          "InfoReturnsNonTimeoutError",
			maxWaitTime:   time.Minute,
			retryInterval: time.Second,
			sleeper:       &MockSleeper{},
			buildkitClientFactory: &fakeBuildkitClientFactoryWithRetries{
				successOnAttempt: 1,
				infoRetriever: &fakeClientInfoRetriever{
					err: []error{assert.AnError},
				},
			},
			expectedErr:          assert.AnError,
			expectedSleeperCalls: 0,
			expectedAttempts:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bw := &buildkitWaiter{
				Logger:                io.NewIOController(),
				buildkitClientFactory: tt.buildkitClientFactory,
				MaxWaitTime:           tt.maxWaitTime,
				RetryInterval:         tt.retryInterval,
				Sleeper:               tt.sleeper,
			}

			err := bw.WaitUntilIsUp(context.Background())
			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.ErrorContains(t, err, tt.expectedErr.Error())
			} else if err != nil {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectedSleeperCalls, tt.sleeper.Calls)
			assert.Equal(t, tt.expectedAttempts, tt.buildkitClientFactory.attempts)
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
				maxBuildkitWaitTimeEnvVar: "",
				retryBuildkitTimeEnvVar:   "",
			},
			expectedMaxWaitTime: maxWaitTime,
			expectedRetryTime:   retryTime,
		},
		{
			name: "CustomMaxWaitTime",
			envVars: map[string]string{
				maxBuildkitWaitTimeEnvVar: "5m",
				retryBuildkitTimeEnvVar:   "",
			},
			expectedMaxWaitTime: 5 * time.Minute,
			expectedRetryTime:   retryTime,
		},
		{
			name: "CustomRetryTime",
			envVars: map[string]string{
				maxBuildkitWaitTimeEnvVar: "",
				retryBuildkitTimeEnvVar:   "10s",
			},
			expectedMaxWaitTime: maxWaitTime,
			expectedRetryTime:   10 * time.Second,
		},
		{
			name: "CustomMaxWaitTimeAndRetryTime",
			envVars: map[string]string{
				maxBuildkitWaitTimeEnvVar: "3m",
				retryBuildkitTimeEnvVar:   "15s",
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

			factory := &ClientFactory{}
			logger := io.NewIOController()
			bw := NewBuildkitClientWaiter(factory, logger)

			assert.Equal(t, tt.expectedMaxWaitTime, bw.MaxWaitTime)
			assert.Equal(t, tt.expectedRetryTime, bw.RetryInterval)
			assert.IsType(t, &RealSleeper{}, bw.Sleeper)
			assert.IsType(t, &buildkitClientFactoryToWait{}, bw.buildkitClientFactory)
			assert.Equal(t, logger, bw.Logger)
		})
	}
}
