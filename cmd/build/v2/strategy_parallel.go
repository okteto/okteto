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
	"slices"

	buildTypes "github.com/okteto/okteto/cmd/build/v2/types"
	"golang.org/x/sync/errgroup"
)

// parallelBuildStrategy builds independent services at the same time, at most
// maxParallel at once. A service is ready once the services it depends on are built.
// When there are more ready services than free slots, the service that unblocks the
// longest chain of dependents goes first, then the one that comes first in svcs (the
// dependency order). The first error cancels the other builds and is returned.
type parallelBuildStrategy struct {
	maxParallel int
}

// maxConcurrency returns the maximum number of builds running at the same time
func (s parallelBuildStrategy) maxConcurrency() int {
	return s.maxParallel
}

// buildResult reports that the build of a service ended
type buildResult struct {
	name string
	ok   bool
}

// Build builds svcs respecting deps. It returns the first build error, or the parent
// context error if it was cancelled.
func (s parallelBuildStrategy) Build(ctx context.Context, svcs []*buildTypes.BuildInfo, deps map[string][]string, build buildServiceFn) error {
	byName := make(map[string]*buildTypes.BuildInfo, len(svcs))
	for _, svc := range svcs {
		byName[svc.Name()] = svc
	}
	dependents := map[string][]string{}
	remaining := make(map[string]int, len(svcs))
	for _, svc := range svcs {
		for _, dep := range deps[svc.Name()] {
			if _, ok := byName[dep]; ok {
				dependents[dep] = append(dependents[dep], svc.Name())
				remaining[svc.Name()]++
			}
		}
	}
	priority := buildPriorities(svcs, dependents)

	var ready []string
	for _, svc := range svcs {
		if remaining[svc.Name()] == 0 {
			ready = append(ready, svc.Name())
		}
	}

	eg, egCtx := errgroup.WithContext(ctx)
	results := make(chan buildResult)
	running := 0
	// failed stops launching builds as soon as one fails, without waiting for errgroup to
	// cancel egCtx, which only happens once the failed build's goroutine returns
	failed := false
	for {
		for running < s.maxParallel && len(ready) > 0 && !failed && egCtx.Err() == nil {
			slices.SortStableFunc(ready, func(a, b string) int { return priority[a] - priority[b] })
			name := ready[0]
			ready = ready[1:]
			running++
			inFlight := running
			eg.Go(func() error {
				err := build(egCtx, byName[name], inFlight)
				results <- buildResult{name: name, ok: err == nil}
				return err
			})
		}
		if running == 0 {
			break
		}
		result := <-results
		running--
		if !result.ok {
			failed = true
			continue
		}
		for _, dependent := range dependents[result.name] {
			remaining[dependent]--
			if remaining[dependent] == 0 {
				ready = append(ready, dependent)
			}
		}
	}
	if err := eg.Wait(); err != nil {
		return err
	}
	return ctx.Err()
}

// buildPriorities ranks svcs for the order in which ready services take free slots:
// lower goes first. Services that unblock a longer chain of dependents go first; ties keep
// the order of svcs.
func buildPriorities(svcs []*buildTypes.BuildInfo, dependents map[string][]string) map[string]int {
	depth := map[string]int{}
	var chainLength func(name string) int
	chainLength = func(name string) int {
		if d, ok := depth[name]; ok {
			return d
		}
		longest := 0
		for _, dependent := range dependents[name] {
			longest = max(longest, 1+chainLength(dependent))
		}
		depth[name] = longest
		return longest
	}

	order := make([]string, 0, len(svcs))
	for _, svc := range svcs {
		order = append(order, svc.Name())
	}
	slices.SortStableFunc(order, func(a, b string) int { return chainLength(b) - chainLength(a) })

	priority := make(map[string]int, len(order))
	for i, name := range order {
		priority[name] = i
	}
	return priority
}
