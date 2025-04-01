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
	"fmt"
	"time"

	"github.com/moby/buildkit/client"
	"github.com/okteto/okteto/pkg/env"
	"github.com/okteto/okteto/pkg/log/io"
)

const (
	// maxWaitTime is the maximum time to wait for buildkit to be available
	maxWaitTime = 10 * time.Minute

	// retryTime is the time to wait between retries
	retryTime = 5 * time.Second

	// maxBuildkitWaitTimeEnvVar is the environment variable to set the max wait time for buildkit
	maxBuildkitWaitTimeEnvVar = "OKTETO_BUILDKIT_WAIT_TIMEOUT"

	// retryBuildkitTimeEnvVar is the environment variable to set the retry time for buildkit
	retryBuildkitIntervalEnvVar = "OKTETO_BUILDKIT_RETRY_INTERVAL"
)

// sleeper defines an interface for sleeping
type sleeper interface {
	Sleep(duration time.Duration)
}

// DefaultSleeper implements the Sleeper interface using time.Sleep
type DefaultSleeper struct{}

// Sleep sleeps for the specified duration
func (s *DefaultSleeper) Sleep(duration time.Duration) {
	time.Sleep(duration)
}

type buildkitClientFactoryWaiter interface {
	GetBuildkitClient(ctx context.Context) (clientInfoRetriever, error)
}

type clientInfoRetriever interface {
	Info(ctx context.Context) (*client.Info, error)
}

// Waiter encapsulates the logic to check if the BuildKit server is up and running
type Waiter struct {
	sleeper               sleeper
	buildkitClientFactory buildkitClientFactoryWaiter
	logger                *io.Controller
	waitingTime           time.Duration
	maxWaitTime           time.Duration
	retryInterval         time.Duration
}

type buildkitClientFactoryToWait struct {
	factory *ClientFactory
}

func (b *buildkitClientFactoryToWait) GetBuildkitClient(ctx context.Context) (clientInfoRetriever, error) {
	return b.factory.GetBuildkitClient(ctx)
}

func buildkitClientFactoryToWaitFactory(factory *ClientFactory) buildkitClientFactoryWaiter {
	return &buildkitClientFactoryToWait{factory}
}

// NewBuildkitClientWaiter creates a new buildkitWaiter
func NewBuildkitClientWaiter(factory *ClientFactory, logger *io.Controller) *Waiter {
	return &Waiter{
		maxWaitTime:           env.LoadTimeOrDefault(maxBuildkitWaitTimeEnvVar, maxWaitTime),
		retryInterval:         env.LoadTimeOrDefault(retryBuildkitIntervalEnvVar, retryTime),
		sleeper:               &DefaultSleeper{},
		buildkitClientFactory: buildkitClientFactoryToWaitFactory(factory),
		logger:                logger,
	}
}

func (bw *Waiter) GetWaitingTime() time.Duration {
	return bw.waitingTime
}

// WaitUntilIsUp waits for the BuildKit server to become available
func (bw *Waiter) WaitUntilIsUp(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, bw.maxWaitTime)
	defer cancel()

	sp := bw.logger.Out().Spinner("Waiting for BuildKit service to be ready...")
	defer sp.Stop()

	startWaitingTime := time.Now()
	for {
		select {
		case <-ctx.Done():
			bw.waitingTime = bw.maxWaitTime
			return fmt.Errorf("buildkit service not available for %v", bw.maxWaitTime)
		default:
			c, err := bw.buildkitClientFactory.GetBuildkitClient(ctx)
			if err != nil {
				sp.Start()
				bw.logger.Infof("Failed to connect to BuildKit service: %v\n", err)
				bw.logger.Infof("Retrying in %v...\n", bw.retryInterval)
				bw.sleeper.Sleep(bw.retryInterval)
				continue
			}
			_, err = c.Info(ctx)
			if err != nil {
				sp.Start()
				bw.logger.Infof("Failed to get BuildKit service info: %v\n", err)
				bw.logger.Infof("Retrying in %v...\n", bw.retryInterval)
				bw.sleeper.Sleep(bw.retryInterval)
				continue
			}
			bw.waitingTime = time.Since(startWaitingTime)
			bw.logger.Infof("Connected to BuildKit service.")
		}
		break
	}

	return nil
}
