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
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	pipelineCMD "github.com/okteto/okteto/cmd/pipeline"
	"github.com/okteto/okteto/pkg/deps"
	"github.com/okteto/okteto/pkg/env"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/validator"
)

var (
	// errDependencyFailed is returned by WaitUntilMainReady when a dependency the main deploy waits for failed.
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
// A failing dependency that the main deploy doesn't wait for doesn't block it: after its failure, only the
// dependencies the main deploy waits for keep starting. After a failure of one of those (or Stop), no new
// dependency is started. The running ones are always allowed to finish.
type dependencyScheduler struct {
	runner           dependencyRunner
	section          deps.ManifestSection
	mainPredecessors map[string]bool
	results          chan dependencyResult
	mainReady        chan struct{}
	done             chan struct{}
	mainReadyErr     error
	err              error
	failed           []string
	mainReadyOnce    sync.Once
	startOnce        sync.Once
	// stopMu is held while a dependency starts, so none starts after Stop returns
	stopMu  sync.Mutex
	stopped bool
}

func newDependencyScheduler(section deps.ManifestSection, runner dependencyRunner) *dependencyScheduler {
	return &dependencyScheduler{
		runner:           runner,
		section:          section,
		mainPredecessors: mainDeployPredecessors(section),
		results:          make(chan dependencyResult, len(section)),
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

// MainPredecessors returns the sorted names of the dependencies the main deploy waits for
func (s *dependencyScheduler) MainPredecessors() []string {
	names := make([]string, 0, len(s.mainPredecessors))
	for name := range s.mainPredecessors {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Failed blocks until every started dependency has finished and returns the sorted names of the ones that failed
func (s *dependencyScheduler) Failed() []string {
	<-s.done
	return s.failed
}

// OverlapsMainDeploy returns true when some dependencies can run at the same time as the main deploy:
// the ones the main deploy doesn't wait for
func (s *dependencyScheduler) OverlapsMainDeploy() bool {
	return len(s.section) > len(s.mainPredecessors)
}

// Stop prevents new dependencies from starting. Running dependencies are allowed to finish.
func (s *dependencyScheduler) Stop() {
	s.stopMu.Lock()
	defer s.stopMu.Unlock()
	s.stopped = true
}

// Wait blocks until every started dependency has finished and returns their errors
func (s *dependencyScheduler) Wait() error {
	<-s.done
	return s.err
}

// coordinate owns the scheduling state: dependencies only report their results through s.results.
// Cancellation is checked before scheduling successors, and no dependency starts after Stop returns.
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
	cancelled := false
	// mainBlocked is set when a dependency the main deploy waits for fails
	mainBlocked := false
	pendingMainPredecessors := len(s.mainPredecessors)

	start := func(name string) {
		s.stopMu.Lock()
		defer s.stopMu.Unlock()
		if s.stopped {
			return
		}
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

			// the pipeline deployer handles Ctrl+C on its own, even when ctx is not cancelled
			if errors.Is(result.err, oktetoErrors.ErrIntSig) {
				if !cancelled {
					cancel()
				}
				continue
			}
			if result.err != nil {
				failures[result.name] = result.err
				if s.mainPredecessors[result.name] {
					mainBlocked = true
					s.resolveMainReady(errDependencyFailed)
				}
				continue
			}
			if s.mainPredecessors[result.name] {
				pendingMainPredecessors--
				if pendingMainPredecessors == 0 {
					s.resolveMainReady(nil)
				}
			}
			if cancelled || mainBlocked {
				continue
			}
			for _, successor := range successors[result.name] {
				pendingPredecessors[successor]--
				if pendingPredecessors[successor] == 0 && s.startsAfterFailures(successor, len(failures) > 0) {
					start(successor)
				}
			}
		}
	}

	for _, name := range names {
		if failures[name] != nil {
			s.failed = append(s.failed, name)
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

// startsAfterFailures returns whether a dependency whose predecessors have succeeded can start when other
// dependencies have failed. Only the dependencies the main deploy waits for keep starting after a failure,
// so a failing dependency that the main deploy doesn't wait for doesn't block it
func (s *dependencyScheduler) startsAfterFailures(name string, failed bool) bool {
	return !failed || s.mainPredecessors[name]
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

// notAwaitedDependencyMinTimeout is the minimum time to wait for the dependencies that the main deploy doesn't
// wait for: before they were never waited for, so a slow one couldn't fail the deploy
const notAwaitedDependencyMinTimeout = 10 * time.Minute

// deployedDependencyEnvVarsGetter returns the env vars generated by a deployed dependency (OKTETO_DEPENDENCY_<NAME>_*)
type deployedDependencyEnvVarsGetter func(ctx context.Context, name, namespace string) (map[string]string, error)

// pipelineDependencyRunner deploys a dependency as a remote pipeline and waits for it to finish
type pipelineDependencyRunner struct {
	pipeline         pipelineCMD.DeployerInterface
	envVarsOf        deployedDependencyEnvVarsGetter
	healthChecked    map[string]bool
	prefixed         map[string]bool
	awaited          map[string]bool
	parentWorkflowID string
	namespace        string
	variables        []string
	// environ is the environment when the dependencies start. Variables are expanded with it and the env vars
	// of the dependencies in depends_on, so that the env vars exported later by the main deploy or by other
	// dependencies don't change the result depending on timing
	environ  []string
	timeout  time.Duration
	redeploy bool
}

func newPipelineDependencyRunner(pipeline pipelineCMD.DeployerInterface, envVarsOf deployedDependencyEnvVarsGetter, section deps.ManifestSection, opts *Options, namespace, parentWorkflowID string) *pipelineDependencyRunner {
	return &pipelineDependencyRunner{
		pipeline:         pipeline,
		envVarsOf:        envVarsOf,
		healthChecked:    healthCheckedDependencies(section, opts.Wait),
		awaited:          mainDeployPredecessors(section),
		parentWorkflowID: parentWorkflowID,
		namespace:        namespace,
		variables:        opts.Variables,
		timeout:          opts.Timeout,
		redeploy:         opts.Dependencies,
		environ:          os.Environ(),
		prefixed:         prefixedDependencies(section),
	}
}

// Run validates and expands the variables of the dependency and deploys it.
// It works on a copy of the dependency because the manifest is shared by the dependencies running in parallel
func (r *pipelineDependencyRunner) Run(ctx context.Context, name string, dep *deps.Dependency) error {
	// prefixed dependencies are announced by their own output, under their own stage
	if !r.prefixed[name] {
		oktetoLog.Information("Deploying dependency '%s'", name)
	}
	d := *dep
	d.Variables = append(env.Environment{}, dep.Variables...)
	if err := validator.CheckReservedVarName(d.Variables); err != nil {
		return err
	}
	d.Variables = append(d.Variables, env.Var{
		Name:  "OKTETO_ORIGIN",
		Value: "okteto-deploy",
	})
	environ, err := r.environWithDependsOn(ctx, d.DependsOn)
	if err != nil {
		return err
	}
	if err := d.ExpandVarsFrom(environ, r.variables); err != nil {
		return fmt.Errorf("could not expand variables in dependencies: %w", err)
	}

	return r.pipeline.ExecuteDeployPipeline(ctx, &pipelineCMD.DeployOptions{
		Name:               name,
		Repository:         d.Repository,
		Branch:             d.Branch,
		File:               d.ManifestPath,
		Variables:          model.SerializeEnvironmentVars(d.Variables),
		Wait:               true,
		SkipResourcesCheck: !r.healthChecked[name],
		PrefixLogs:         r.prefixed[name],
		Timeout:            r.dependencyTimeout(name, &d),
		SkipIfExists:       !r.redeploy,
		Namespace:          r.namespace,
		IsDependency:       true,
		ParentWorkflowID:   r.parentWorkflowID,
	})
}

// environWithDependsOn returns the start environment plus the env vars generated by the dependencies in
// depends_on: they have all been deployed successfully before the dependency starts. The predecessors of those
// are left out, so a dependency only uses the env vars of the dependencies it declares
func (r *pipelineDependencyRunner) environWithDependsOn(ctx context.Context, dependsOn []string) ([]string, error) {
	environ := append([]string{}, r.environ...)
	for _, predecessor := range dependsOn {
		envs, err := r.envVarsOf(ctx, predecessor, r.namespace)
		if err != nil {
			return nil, err
		}
		keys := make([]string, 0, len(envs))
		for key := range envs {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			environ = append(environ, fmt.Sprintf("%s=%s", key, envs[key]))
		}
	}
	return environ, nil
}

// dependencyTimeout returns how long to wait for the dependency: its own timeout if it's set. Otherwise the
// --timeout flag, with a minimum of notAwaitedDependencyMinTimeout for the dependencies the main deploy doesn't wait for
func (r *pipelineDependencyRunner) dependencyTimeout(name string, dep *deps.Dependency) time.Duration {
	timeout := r.timeout
	if !r.awaited[name] {
		timeout = max(timeout, notAwaitedDependencyMinTimeout)
	}
	return dep.GetTimeout(timeout)
}

// prefixedDependencies returns the dependencies whose logs are prefixed with their name: all of them when
// there are several, because they run in parallel. A single dependency keeps the current output (spinner and
// no prefix) unless it runs at the same time as the main deploy, because the main deploy doesn't wait for it
func prefixedDependencies(section deps.ManifestSection) map[string]bool {
	prefixed := map[string]bool{}
	mainPredecessors := mainDeployPredecessors(section)
	for name := range section {
		prefixed[name] = len(section) > 1 || !mainPredecessors[name]
	}
	return prefixed
}

// healthCheckedDependencies returns the dependencies that must have their resources running to succeed:
// the predecessors of the main deploy or of another dependency, and all of them with --wait
func healthCheckedDependencies(section deps.ManifestSection, waitFlag bool) map[string]bool {
	healthChecked := mainDeployPredecessors(section)
	for name, dep := range section {
		if waitFlag {
			healthChecked[name] = true
		}
		for _, predecessor := range dep.DependsOn {
			healthChecked[predecessor] = true
		}
	}
	return healthChecked
}
