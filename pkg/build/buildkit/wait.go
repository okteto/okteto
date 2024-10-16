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
	retryBuildkitIntervalEnvVar = "OKTETO_RETRY_BUILDKIT_INTERVAL"
)

// sleeper defines an interface for sleeping
type sleeper interface {
	Sleep(duration time.Duration)
}

// RealSleeper implements the Sleeper interface using time.Sleep
type RealSleeper struct{}

// Sleep sleeps for the specified duration
func (s *RealSleeper) Sleep(duration time.Duration) {
	time.Sleep(duration)
}

type buildkitClientFactoryWaiter interface {
	GetBuildkitClient(ctx context.Context) (clientInfoRetriever, error)
}

type clientInfoRetriever interface {
	Info(ctx context.Context) (*client.Info, error)
}

// buildkitWaiter encapsulates the logic to check if the BuildKit server is up and running
type buildkitWaiter struct {
	Sleeper               sleeper
	buildkitClientFactory buildkitClientFactoryWaiter
	Logger                *io.Controller
	waitingTime           time.Duration
	MaxWaitTime           time.Duration
	RetryInterval         time.Duration
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
func NewBuildkitClientWaiter(factory *ClientFactory, logger *io.Controller) *buildkitWaiter {
	return &buildkitWaiter{
		MaxWaitTime:           env.LoadTimeOrDefault(maxBuildkitWaitTimeEnvVar, maxWaitTime),
		RetryInterval:         env.LoadTimeOrDefault(retryBuildkitIntervalEnvVar, retryTime),
		Sleeper:               &RealSleeper{},
		buildkitClientFactory: buildkitClientFactoryToWaitFactory(factory),
		Logger:                logger,
	}
}

func (bw *buildkitWaiter) GetWaitingTime() time.Duration {
	return bw.waitingTime
}

// WaitForBuildKit waits for the BuildKit server to become available
func (bw *buildkitWaiter) WaitUntilIsUp(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, bw.MaxWaitTime)
	defer cancel()

	sp := bw.Logger.Out().Spinner("Waiting for BuildKit service to be ready...")
	sp.Start()
	defer sp.Stop()

	startWaitingTime := time.Now()
	for {
		select {
		case <-ctx.Done():
			bw.waitingTime = bw.MaxWaitTime
			return fmt.Errorf("buildkit service not available for %v", bw.MaxWaitTime)
		default:
			c, err := bw.buildkitClientFactory.GetBuildkitClient(ctx)
			if err != nil {
				bw.Logger.Infof("Failed to connect to BuildKit service: %v\n", err)
				bw.Logger.Infof("Retrying in %v...\n", bw.RetryInterval)
				bw.Sleeper.Sleep(bw.RetryInterval)
				continue
			}
			_, err = c.Info(ctx)
			if err != nil {
				bw.Logger.Infof("Failed to get BuildKit service info: %v\n", err)
				bw.Logger.Infof("Retrying in %v...\n", bw.RetryInterval)
				bw.Sleeper.Sleep(bw.RetryInterval)
				continue
			}
			bw.waitingTime = time.Since(startWaitingTime)
			bw.Logger.Infof("Connected to BuildKit service.")
		}
		break
	}

	return nil
}
