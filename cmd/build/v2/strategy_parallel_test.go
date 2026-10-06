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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	buildTypes "github.com/okteto/okteto/cmd/build/v2/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// eventLog records build events in order
type eventLog struct {
	events []string
	mu     sync.Mutex
}

func (l *eventLog) add(event string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
}

func (l *eventLog) index(event string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i, e := range l.events {
		if e == event {
			return i
		}
	}
	return -1
}

// maxTracker tracks the maximum number of builds running at the same time
type maxTracker struct {
	running atomic.Int32
	max     atomic.Int32
}

// start records a build start and returns the function that records its end
func (m *maxTracker) start() func() {
	n := m.running.Add(1)
	for {
		current := m.max.Load()
		if n <= current || m.max.CompareAndSwap(current, n) {
			break
		}
	}
	return func() { m.running.Add(-1) }
}

// doneOrNever returns a closed channel when done is true, or ctx.Done() otherwise
func doneOrNever(ctx context.Context, done bool) <-chan struct{} {
	if done {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return ctx.Done()
}

func TestParallelBuildStrategyWaitsForDependencies(t *testing.T) {
	svcs := buildTypes.NewBuildInfos("manifest", "ns", "repo", []string{"a", "b"})
	log := &eventLog{}

	err := parallelBuildStrategy{maxParallel: 4}.Build(context.Background(), svcs, map[string][]string{"b": {"a"}}, func(_ context.Context, svc *buildTypes.BuildInfo, _ int) error {
		log.add("start " + svc.Name())
		time.Sleep(10 * time.Millisecond)
		log.add("end " + svc.Name())
		return nil
	})

	require.NoError(t, err)
	require.Less(t, log.index("end a"), log.index("start b"))
}

func TestParallelBuildStrategyRespectsMaxParallel(t *testing.T) {
	svcs := buildTypes.NewBuildInfos("manifest", "ns", "repo", []string{"a", "b", "c", "d", "e", "f"})
	tracker := &maxTracker{}

	err := parallelBuildStrategy{maxParallel: 2}.Build(context.Background(), svcs, nil, func(context.Context, *buildTypes.BuildInfo, int) error {
		defer tracker.start()()
		time.Sleep(20 * time.Millisecond)
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, int32(2), tracker.max.Load())
}

func TestParallelBuildStrategyStartsNextWhenASlotIsFree(t *testing.T) {
	svcs := buildTypes.NewBuildInfos("manifest", "ns", "repo", []string{"slow", "a", "b"})
	durations := map[string]time.Duration{"slow": 300 * time.Millisecond, "a": 10 * time.Millisecond, "b": 10 * time.Millisecond}
	log := &eventLog{}

	err := parallelBuildStrategy{maxParallel: 2}.Build(context.Background(), svcs, nil, func(_ context.Context, svc *buildTypes.BuildInfo, _ int) error {
		log.add("start " + svc.Name())
		time.Sleep(durations[svc.Name()])
		log.add("end " + svc.Name())
		return nil
	})

	require.NoError(t, err)
	require.Less(t, log.index("start a"), log.index("end slow"))
	require.Less(t, log.index("start b"), log.index("end slow"))
}

func TestParallelBuildStrategyFailFastSkipsDependents(t *testing.T) {
	svcs := buildTypes.NewBuildInfos("manifest", "ns", "repo", []string{"a", "b"})
	errs := map[string]error{"a": assert.AnError}
	log := &eventLog{}

	err := parallelBuildStrategy{maxParallel: 4}.Build(context.Background(), svcs, map[string][]string{"b": {"a"}}, func(_ context.Context, svc *buildTypes.BuildInfo, _ int) error {
		log.add("start " + svc.Name())
		return errs[svc.Name()]
	})

	require.ErrorIs(t, err, assert.AnError)
	require.Equal(t, -1, log.index("start b"))
}

func TestParallelBuildStrategyCancelsRunningBuildsOnFailure(t *testing.T) {
	svcs := buildTypes.NewBuildInfos("manifest", "ns", "repo", []string{"a", "b"})
	errs := map[string]error{"a": assert.AnError}
	failsRightAway := map[string]bool{"a": true}
	delays := map[string]time.Duration{"a": 20 * time.Millisecond}
	var cancelled atomic.Bool

	err := parallelBuildStrategy{maxParallel: 4}.Build(context.Background(), svcs, nil, func(ctx context.Context, svc *buildTypes.BuildInfo, _ int) error {
		time.Sleep(delays[svc.Name()])
		<-doneOrNever(ctx, failsRightAway[svc.Name()])
		cancelled.Store(ctx.Err() != nil)
		return errs[svc.Name()]
	})

	require.ErrorIs(t, err, assert.AnError)
	require.True(t, cancelled.Load())
}

func TestParallelBuildStrategyReturnsParentCancellation(t *testing.T) {
	svcs := buildTypes.NewBuildInfos("manifest", "ns", "repo", []string{"a", "b"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var calls atomic.Int32

	err := parallelBuildStrategy{maxParallel: 4}.Build(ctx, svcs, nil, func(context.Context, *buildTypes.BuildInfo, int) error {
		calls.Add(1)
		return nil
	})

	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, int32(0), calls.Load())
}

func TestParallelBuildStrategyReportsBuildsInFlight(t *testing.T) {
	svcs := buildTypes.NewBuildInfos("manifest", "ns", "repo", []string{"a", "b"})
	var started sync.WaitGroup
	started.Add(2)
	var seen sync.Map

	err := parallelBuildStrategy{maxParallel: 4}.Build(context.Background(), svcs, nil, func(_ context.Context, _ *buildTypes.BuildInfo, inFlight int) error {
		seen.Store(inFlight, true)
		started.Done()
		started.Wait()
		return nil
	})

	require.NoError(t, err)
	_, sawOne := seen.Load(1)
	_, sawTwo := seen.Load(2)
	require.True(t, sawOne)
	require.True(t, sawTwo)
}

// startOrder returns the services in the order they started
func (l *eventLog) startOrder() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.events)
}

func buildOneAtATime(t *testing.T, names []string, deps map[string][]string) []string {
	t.Helper()
	log := &eventLog{}
	err := parallelBuildStrategy{maxParallel: 1}.Build(context.Background(), buildTypes.NewBuildInfos("manifest", "ns", "repo", names), deps, func(_ context.Context, svc *buildTypes.BuildInfo, _ int) error {
		log.add(svc.Name())
		return nil
	})
	require.NoError(t, err)
	return log.startOrder()
}

func TestParallelBuildStrategyStartsServicesWithDependentsFirst(t *testing.T) {
	order := buildOneAtATime(t, []string{"web", "worker", "base", "api"}, map[string][]string{"api": {"base"}})

	require.Equal(t, []string{"base", "web", "worker", "api"}, order)
}

func TestParallelBuildStrategyPrioritizesTheLongestChain(t *testing.T) {
	order := buildOneAtATime(t, []string{"x", "a", "b", "c"}, map[string][]string{"b": {"a"}, "c": {"b"}})

	require.Equal(t, []string{"a", "b", "x", "c"}, order)
}

func TestParallelBuildStrategyKeepsOrderOfIndependentServices(t *testing.T) {
	order := buildOneAtATime(t, []string{"a", "b", "c"}, nil)

	require.Equal(t, []string{"a", "b", "c"}, order)
}
