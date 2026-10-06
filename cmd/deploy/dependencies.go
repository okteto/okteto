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

package deploy

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/okteto/okteto/pkg/deps"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
)

var (
	// errDependencyFailed is returned by WaitUntilMainReady when a dependency failed before the main deploy could start.
	// The details of every failure are returned by Wait.
	errDependencyFailed = errors.New("a dependency failed to deploy")

	// errDependenciesStopped is returned by WaitUntilMainReady when Stop was called before the main deploy could start
	errDependenciesStopped = errors.New("the dependencies deployment was stopped")
)

// dependencyRunner deploys a single dependency and returns when it has finished
type dependencyRunner interface {
	Run(ctx context.Context, name string, dep *deps.Dependency) error
}

type dependencyResult struct {
	err  error
	name string
}

// dependencyScheduler deploys the dependencies of a manifest following their depends_on graph.
// A dependency starts as soon as all its predecessors have succeeded. The main deploy is treated
// as one more node: it is ready when its predecessors (every dependency with wait: true, and all
// their predecessors) have succeeded.
// After the first failure (or Stop), no new dependency is started and the running ones are
// allowed to finish.
type dependencyScheduler struct {
	runner           dependencyRunner
	section          deps.ManifestSection
	mainPredecessors map[string]bool
	results          chan dependencyResult
	stop             chan struct{}
	mainReady        chan struct{}
	done             chan struct{}
	mainReadyErr     error
	err              error
	stopOnce         sync.Once
	mainReadyOnce    sync.Once
	startOnce        sync.Once
}

func newDependencyScheduler(section deps.ManifestSection, runner dependencyRunner) *dependencyScheduler {
	return &dependencyScheduler{
		runner:           runner,
		section:          section,
		mainPredecessors: mainDeployPredecessors(section),
		results:          make(chan dependencyResult, len(section)),
		stop:             make(chan struct{}),
		mainReady:        make(chan struct{}),
		done:             make(chan struct{}),
	}
}

// Start launches the dependencies graph and returns immediately. It must be called before
// WaitUntilMainReady or Wait, and calling it more than once has no effect.
func (s *dependencyScheduler) Start(ctx context.Context) {
	s.startOnce.Do(func() {
		go s.coordinate(ctx)
	})
}

// WaitUntilMainReady blocks until the main deploy can start, a dependency failed or the context is cancelled
func (s *dependencyScheduler) WaitUntilMainReady() error {
	<-s.mainReady
	return s.mainReadyErr
}

// Stop prevents new dependencies from starting. Running dependencies are allowed to finish.
func (s *dependencyScheduler) Stop() {
	s.stopOnce.Do(func() {
		close(s.stop)
	})
}

// Wait blocks until every started dependency has finished and returns their errors
func (s *dependencyScheduler) Wait() error {
	<-s.done
	return s.err
}

// coordinate owns the scheduling state: dependencies only report their results through s.results.
// Stop and cancellation are checked before scheduling successors, so a result that arrives at the
// same time as Stop or Ctrl+C never starts new dependencies.
func (s *dependencyScheduler) coordinate(ctx context.Context) {
	names := sortedDependencyNames(s.section)
	pendingPredecessors := make(map[string]int, len(s.section))
	successors := make(map[string][]string, len(s.section))
	for _, name := range names {
		pendingPredecessors[name] = len(s.section[name].DependsOn)
		for _, predecessor := range s.section[name].DependsOn {
			successors[predecessor] = append(successors[predecessor], name)
		}
	}

	started := make(map[string]bool, len(s.section))
	failures := map[string]error{}
	running := 0
	stopped := false
	cancelled := false
	pendingMainPredecessors := len(s.mainPredecessors)

	start := func(name string) {
		started[name] = true
		running++
		go func() {
			s.results <- dependencyResult{name: name, err: s.runner.Run(ctx, name, s.section[name])}
		}()
	}
	cancel := func() {
		cancelled = true
		s.resolveMainReady(oktetoErrors.ErrIntSig)
	}

	if pendingMainPredecessors == 0 {
		s.resolveMainReady(nil)
	}
	for _, name := range names {
		if pendingPredecessors[name] == 0 {
			start(name)
		}
	}

	ctxDone := ctx.Done()
	for running > 0 {
		select {
		case <-ctxDone:
			ctxDone = nil
			cancel()
		case result := <-s.results:
			running--
			if !cancelled && ctx.Err() != nil {
				cancel()
			}
			if s.isStopped() {
				stopped = true
			}

			// the pipeline deployer handles Ctrl+C on its own, even when ctx is not cancelled
			if errors.Is(result.err, oktetoErrors.ErrIntSig) {
				if !cancelled {
					cancel()
				}
				continue
			}
			if result.err != nil {
				failures[result.name] = result.err
				s.resolveMainReady(errDependencyFailed)
				continue
			}
			if s.mainPredecessors[result.name] {
				pendingMainPredecessors--
				if pendingMainPredecessors == 0 {
					s.resolveMainReady(nil)
				}
			}
			if cancelled || stopped || len(failures) > 0 {
				continue
			}
			for _, successor := range successors[result.name] {
				pendingPredecessors[successor]--
				if pendingPredecessors[successor] == 0 {
					start(successor)
				}
			}
		}
	}

	notStarted := notStartedDependencies(names, started)
	switch {
	case cancelled:
		s.err = errors.Join(oktetoErrors.ErrIntSig, dependenciesError(failures, nil, ""))
	case len(failures) > 0:
		s.err = dependenciesError(failures, notStarted, "because of a previous failure")
	default:
		s.err = dependenciesError(failures, notStarted, "because the deploy was stopped")
	}
	if len(failures) > 0 {
		s.resolveMainReady(errDependencyFailed)
	}
	s.resolveMainReady(errDependenciesStopped)
	close(s.done)
}

func (s *dependencyScheduler) isStopped() bool {
	select {
	case <-s.stop:
		return true
	default:
		return false
	}
}

func (s *dependencyScheduler) resolveMainReady(err error) {
	s.mainReadyOnce.Do(func() {
		s.mainReadyErr = err
		close(s.mainReady)
	})
}

// mainDeployPredecessors returns the dependencies the main deploy waits for:
// the ones with wait: true and all their predecessors
func mainDeployPredecessors(section deps.ManifestSection) map[string]bool {
	predecessors := map[string]bool{}
	var add func(name string)
	add = func(name string) {
		if predecessors[name] {
			return
		}
		predecessors[name] = true
		for _, predecessor := range section[name].DependsOn {
			add(predecessor)
		}
	}
	for name, dep := range section {
		if dep.Wait {
			add(name)
		}
	}
	return predecessors
}

func dependenciesError(failures map[string]error, notStarted []string, notStartedReason string) error {
	names := make([]string, 0, len(failures))
	for name := range failures {
		names = append(names, name)
	}
	sort.Strings(names)

	errs := make([]error, 0, len(names)+1)
	for _, name := range names {
		errs = append(errs, fmt.Errorf("dependency '%s' failed: %w", name, failures[name]))
	}
	if len(notStarted) > 0 {
		errs = append(errs, fmt.Errorf("dependencies not deployed %s: %s", notStartedReason, strings.Join(notStarted, ", ")))
	}
	return errors.Join(errs...)
}

func notStartedDependencies(names []string, started map[string]bool) []string {
	notStarted := []string{}
	for _, name := range names {
		if !started[name] {
			notStarted = append(notStarted, name)
		}
	}
	return notStarted
}

func sortedDependencyNames(section deps.ManifestSection) []string {
	names := make([]string, 0, len(section))
	for name := range section {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
