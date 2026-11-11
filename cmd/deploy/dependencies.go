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
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/validator"
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
// A dependency starts as soon as all its predecessors have succeeded. After a failure or Ctrl+C no new
// dependency is started, and the running ones are allowed to finish.
type dependencyScheduler struct {
	runner    dependencyRunner
	section   deps.ManifestSection
	results   chan dependencyResult
	done      chan struct{}
	err       error
	startOnce sync.Once
}

func newDependencyScheduler(section deps.ManifestSection, runner dependencyRunner) *dependencyScheduler {
	return &dependencyScheduler{
		runner:  runner,
		section: section,
		results: make(chan dependencyResult, len(section)),
		done:    make(chan struct{}),
	}
}

// Start launches the dependencies graph and returns immediately. It must be called before Wait, and calling it
// more than once has no effect.
func (s *dependencyScheduler) Start(ctx context.Context) {
	s.startOnce.Do(func() {
		go s.coordinate(ctx)
	})
}

// Wait blocks until every started dependency has finished and returns their errors
func (s *dependencyScheduler) Wait() error {
	<-s.done
	return s.err
}

// coordinate owns the scheduling state: dependencies only report their results through s.results.
// Cancellation is checked before scheduling successors.
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

	start := func(name string) {
		started[name] = true
		running++
		go func() {
			s.results <- dependencyResult{name: name, err: s.runner.Run(ctx, name, s.section[name])}
		}()
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
			cancelled = true
		case result := <-s.results:
			running--
			if ctx.Err() != nil {
				cancelled = true
			}

			// the pipeline deployer handles Ctrl+C on its own, even when ctx is not cancelled
			if errors.Is(result.err, oktetoErrors.ErrIntSig) {
				cancelled = true
				continue
			}
			if result.err != nil {
				failures[result.name] = result.err
				continue
			}
			if cancelled || len(failures) > 0 {
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

	if cancelled {
		s.err = errors.Join(oktetoErrors.ErrIntSig, dependenciesError(failures, nil))
	} else {
		s.err = dependenciesError(failures, notStartedDependencies(names, started))
	}
	close(s.done)
}

// mainDeployPredecessors returns the dependencies the deploy commands depend on:
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

// awaitedDependencies returns the dependencies that the deploy waits for: the ones with wait: true, their
// predecessors, and every dependency that another one lists in depends_on. The rest are only scheduled
func awaitedDependencies(section deps.ManifestSection) map[string]bool {
	awaited := map[string]bool{}
	for name, dep := range section {
		if dep.Wait {
			awaited[name] = true
		}
		// the predecessors of a wait: true dependency are in the depends_on of the dependencies in its chain
		for _, predecessor := range dep.DependsOn {
			awaited[predecessor] = true
		}
	}
	return awaited
}

// dependenciesError joins the failures and, after a failure, lists the dependencies that didn't start
func dependenciesError(failures map[string]error, notStarted []string) error {
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
		errs = append(errs, fmt.Errorf("dependencies not deployed because of a previous failure: %s", strings.Join(notStarted, ", ")))
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

// deployedDependencyEnvVarsGetter returns the env vars generated by a deployed dependency (OKTETO_DEPENDENCY_<NAME>_*)
type deployedDependencyEnvVarsGetter func(ctx context.Context, name, namespace string) (map[string]string, error)

// pipelineDependencyRunner deploys a dependency as a remote pipeline. It waits for the awaited dependencies to be
// deployed and their resources to be running, and only schedules the rest. The messages and logs of each
// dependency are printed with its name
type pipelineDependencyRunner struct {
	pipeline         pipelineCMD.DeployerInterface
	envVarsOf        deployedDependencyEnvVarsGetter
	awaited          map[string]bool
	parentWorkflowID string
	namespace        string
	variables        []string
	// environ is the environment when the dependencies start. Variables are expanded with it and the env vars
	// of the dependencies in depends_on, so that the result doesn't depend on when each dependency starts
	environ  []string
	timeout  time.Duration
	redeploy bool
}

func newPipelineDependencyRunner(pipeline pipelineCMD.DeployerInterface, envVarsOf deployedDependencyEnvVarsGetter, section deps.ManifestSection, opts *Options, namespace, parentWorkflowID string) *pipelineDependencyRunner {
	return &pipelineDependencyRunner{
		pipeline:         pipeline,
		envVarsOf:        envVarsOf,
		awaited:          awaitedDependencies(section),
		parentWorkflowID: parentWorkflowID,
		namespace:        namespace,
		variables:        opts.Variables,
		timeout:          opts.Timeout,
		redeploy:         opts.Dependencies,
		environ:          os.Environ(),
	}
}

// Run validates and expands the variables of the dependency and deploys it.
// It works on a copy of the dependency because the manifest is shared by the dependencies running in parallel
func (r *pipelineDependencyRunner) Run(ctx context.Context, name string, dep *deps.Dependency) (err error) {
	out := newDependencyOutput(name, r.awaited[name])
	defer func() { out.finish(err) }()

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
		Name:             name,
		Repository:       d.Repository,
		Branch:           d.Branch,
		File:             d.ManifestPath,
		Variables:        model.SerializeEnvironmentVars(d.Variables),
		Wait:             r.awaited[name],
		Output:           out,
		Timeout:          d.GetTimeout(r.timeout),
		SkipIfExists:     !r.redeploy,
		Namespace:        r.namespace,
		IsDependency:     true,
		ParentWorkflowID: r.parentWorkflowID,
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
