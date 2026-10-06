// Copyright 2026 The Okteto Authors
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

package v2

import (
	"context"
	"sync/atomic"

	buildTypes "github.com/okteto/okteto/cmd/build/v2/types"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

// parallelBuildStrategy builds independent services at the same time, at most
// maxParallel at once. A service starts as soon as the services it depends on are built
// and a slot is free. The first error cancels the other builds and is returned.
type parallelBuildStrategy struct {
	maxParallel int
}

// Build builds svcs respecting deps. It returns the first build error, or the parent
// context error if it was cancelled.
func (s parallelBuildStrategy) Build(ctx context.Context, svcs []*buildTypes.BuildInfo, deps map[string][]string, build buildServiceFn) error {
	done := make(map[string]chan struct{}, len(svcs))
	for _, svc := range svcs {
		done[svc.Name()] = make(chan struct{})
	}
	slots := semaphore.NewWeighted(int64(s.maxParallel))
	var inFlight atomic.Int32

	eg, egCtx := errgroup.WithContext(ctx)
	for _, svc := range svcs {
		eg.Go(func() error {
			if !waitForDependencies(egCtx, deps[svc.Name()], done) {
				return nil
			}
			if err := slots.Acquire(egCtx, 1); err != nil {
				return nil
			}
			defer slots.Release(1)
			n := int(inFlight.Add(1))
			defer inFlight.Add(-1)

			if err := build(egCtx, svc, n); err != nil {
				return err
			}
			close(done[svc.Name()])
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}
	return ctx.Err()
}

// waitForDependencies waits until every dependency is built. It returns false when ctx is
// done first, meaning the service must not be built.
func waitForDependencies(ctx context.Context, deps []string, done map[string]chan struct{}) bool {
	for _, dep := range deps {
		depDone, ok := done[dep]
		if !ok {
			continue
		}
		select {
		case <-depDone:
		case <-ctx.Done():
			return false
		}
	}
	return ctx.Err() == nil
}
