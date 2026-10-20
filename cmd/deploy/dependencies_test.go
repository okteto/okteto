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
	"sync"
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/deps"
	"github.com/okteto/okteto/pkg/env"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/validator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const schedulerTestTimeout = 5 * time.Second

// fakeDependencyRunner blocks every dependency until the test finishes it,
// so tests decide the exact order in which dependencies complete.
type fakeDependencyRunner struct {
	started  chan string
	releases map[string]chan error
	events   []string
	mu       sync.Mutex
}

func newFakeDependencyRunner(section deps.ManifestSection) *fakeDependencyRunner {
	releases := make(map[string]chan error, len(section))
	for name := range section {
		releases[name] = make(chan error, 1)
	}
	return &fakeDependencyRunner{
		started:  make(chan string, len(section)),
		releases: releases,
	}
}

func (f *fakeDependencyRunner) Run(ctx context.Context, name string, _ *deps.Dependency) error {
	f.record("start:" + name)
	f.started <- name
	select {
	case err := <-f.releases[name]:
		f.record("end:" + name)
		return err
	case <-ctx.Done():
		f.record("end:" + name)
		// a result released before the cancellation wins, as it would in a real deploy
		select {
		case err := <-f.releases[name]:
			return err
		default:
			return oktetoErrors.ErrIntSig
		}
	}
}

func (f *fakeDependencyRunner) record(event string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, event)
}

func (f *fakeDependencyRunner) finish(name string, err error) {
	f.releases[name] <- err
}

func (f *fakeDependencyRunner) indexOf(t *testing.T, event string) int {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, e := range f.events {
		if e == event {
			return i
		}
	}
	require.FailNow(t, "event not recorded", event)
	return -1
}

func requireStarted(t *testing.T, f *fakeDependencyRunner, names ...string) {
	t.Helper()
	got := make([]string, 0, len(names))
	for range names {
		select {
		case name := <-f.started:
			got = append(got, name)
		case <-time.After(schedulerTestTimeout):
			require.FailNow(t, "timed out waiting for dependencies to start", "expected %v, got %v", names, got)
		}
	}
	require.ElementsMatch(t, names, got)
}

// requireNoMoreStarted must be called after Wait returns: every started dependency has finished by then
func requireNoMoreStarted(t *testing.T, f *fakeDependencyRunner) {
	t.Helper()
	require.Empty(t, f.started)
}

func requireMainReadyResult(t *testing.T, mainReady <-chan error) error {
	t.Helper()
	select {
	case err := <-mainReady:
		return err
	case <-time.After(schedulerTestTimeout):
		require.FailNow(t, "timed out waiting for the main deploy to be ready")
		return nil
	}
}

func requireMainReadyPending(t *testing.T, mainReady <-chan error) {
	t.Helper()
	select {
	case err := <-mainReady:
		require.FailNow(t, "main deploy ready too early", "err: %v", err)
	default:
	}
}

func waitUntilMainReadyAsync(s *dependencyScheduler) <-chan error {
	mainReady := make(chan error, 1)
	go func() {
		mainReady <- s.WaitUntilMainReady()
	}()
	return mainReady
}

func requireWaitResult(t *testing.T, s *dependencyScheduler) error {
	t.Helper()
	result := make(chan error, 1)
	go func() {
		result <- s.Wait()
	}()
	select {
	case err := <-result:
		return err
	case <-time.After(schedulerTestTimeout):
		require.FailNow(t, "timed out waiting for the dependencies to finish")
		return nil
	}
}

// specExampleSection is the example from the product spec:
// database and messaging are roots, backend waits for both, analytics waits for database.
func specExampleSection() deps.ManifestSection {
	return deps.ManifestSection{
		"database":  &deps.Dependency{Repository: "https://github.com/okteto/database"},
		"messaging": &deps.Dependency{Repository: "https://github.com/okteto/messaging"},
		"backend": &deps.Dependency{
			Repository: "https://github.com/okteto/backend",
			DependsOn:  deps.DependsOn{"database", "messaging"},
			Wait:       true,
		},
		"analytics": &deps.Dependency{
			Repository: "https://github.com/okteto/analytics",
			DependsOn:  deps.DependsOn{"database"},
		},
	}
}

func TestDependencyScheduler_SpecExample(t *testing.T) {
	section := specExampleSection()
	runner := newFakeDependencyRunner(section)
	s := newDependencyScheduler(section, runner)

	s.Start(context.Background())
	mainReady := waitUntilMainReadyAsync(s)

	requireStarted(t, runner, "database", "messaging")

	runner.finish("database", nil)
	requireStarted(t, runner, "analytics")

	runner.finish("messaging", nil)
	requireStarted(t, runner, "backend")
	requireMainReadyPending(t, mainReady)

	runner.finish("backend", nil)
	require.NoError(t, requireMainReadyResult(t, mainReady))

	runner.finish("analytics", nil)
	require.NoError(t, requireWaitResult(t, s))
	requireNoMoreStarted(t, runner)

	require.Less(t, runner.indexOf(t, "end:database"), runner.indexOf(t, "start:analytics"))
	require.Less(t, runner.indexOf(t, "end:database"), runner.indexOf(t, "start:backend"))
	require.Less(t, runner.indexOf(t, "end:messaging"), runner.indexOf(t, "start:backend"))
}

func TestDependencyScheduler_MainReadyWaitsForAncestorsOfWaitDependencies(t *testing.T) {
	section := deps.ManifestSection{
		"database": &deps.Dependency{Repository: "https://github.com/okteto/database"},
		"backend": &deps.Dependency{
			Repository: "https://github.com/okteto/backend",
			DependsOn:  deps.DependsOn{"database"},
			Wait:       true,
		},
	}
	runner := newFakeDependencyRunner(section)
	s := newDependencyScheduler(section, runner)

	s.Start(context.Background())
	mainReady := waitUntilMainReadyAsync(s)

	requireStarted(t, runner, "database")
	requireMainReadyPending(t, mainReady)
	runner.finish("database", nil)

	requireStarted(t, runner, "backend")
	requireMainReadyPending(t, mainReady)
	runner.finish("backend", nil)

	require.NoError(t, requireMainReadyResult(t, mainReady))
	require.NoError(t, requireWaitResult(t, s))
}

func TestDependencyScheduler_MainReadyImmediatelyWithoutWaitDependencies(t *testing.T) {
	section := deps.ManifestSection{
		"database": &deps.Dependency{Repository: "https://github.com/okteto/database"},
	}
	runner := newFakeDependencyRunner(section)
	s := newDependencyScheduler(section, runner)

	s.Start(context.Background())

	require.NoError(t, requireMainReadyResult(t, waitUntilMainReadyAsync(s)))
	requireStarted(t, runner, "database")

	runner.finish("database", nil)
	require.NoError(t, requireWaitResult(t, s))
}

func TestDependencyScheduler_NoDependencies(t *testing.T) {
	section := deps.ManifestSection{}
	s := newDependencyScheduler(section, newFakeDependencyRunner(section))

	s.Start(context.Background())

	require.NoError(t, requireMainReadyResult(t, waitUntilMainReadyAsync(s)))
	require.NoError(t, requireWaitResult(t, s))
}

func TestDependencyScheduler_FailureBeforeMainReady(t *testing.T) {
	section := specExampleSection()
	runner := newFakeDependencyRunner(section)
	s := newDependencyScheduler(section, runner)

	s.Start(context.Background())
	mainReady := waitUntilMainReadyAsync(s)
	requireStarted(t, runner, "database", "messaging")

	runner.finish("messaging", errors.New("boom"))
	require.ErrorIs(t, requireMainReadyResult(t, mainReady), errDependencyFailed)

	// database was already running, so it is allowed to finish, but nothing new starts
	runner.finish("database", nil)
	err := requireWaitResult(t, s)
	requireNoMoreStarted(t, runner)

	require.EqualError(t, err, "dependency 'messaging' failed: boom\ndependencies not deployed because of a previous failure: analytics, backend")
}

func TestDependencyScheduler_FailureAfterMainReady(t *testing.T) {
	section := specExampleSection()
	runner := newFakeDependencyRunner(section)
	s := newDependencyScheduler(section, runner)

	s.Start(context.Background())
	mainReady := waitUntilMainReadyAsync(s)
	requireStarted(t, runner, "database", "messaging")
	runner.finish("database", nil)
	requireStarted(t, runner, "analytics")
	runner.finish("messaging", nil)
	requireStarted(t, runner, "backend")
	runner.finish("backend", nil)
	require.NoError(t, requireMainReadyResult(t, mainReady))

	runner.finish("analytics", errors.New("boom"))

	require.EqualError(t, requireWaitResult(t, s), "dependency 'analytics' failed: boom")
}

func TestDependencyScheduler_FailedDependencies(t *testing.T) {
	section := deps.ManifestSection{
		"database":  &deps.Dependency{Repository: "https://github.com/okteto/database"},
		"messaging": &deps.Dependency{Repository: "https://github.com/okteto/messaging"},
		"reports":   &deps.Dependency{Repository: "https://github.com/okteto/reports"},
	}
	runner := newFakeDependencyRunner(section)
	s := newDependencyScheduler(section, runner)

	s.Start(context.Background())
	requireStarted(t, runner, "database", "messaging", "reports")
	runner.finish("reports", errors.New("boom"))
	runner.finish("messaging", errors.New("boom"))
	runner.finish("database", nil)
	require.Error(t, requireWaitResult(t, s))

	require.Equal(t, []string{"messaging", "reports"}, s.Failed())
}

func TestDependencyScheduler_ReportsEveryFailure(t *testing.T) {
	section := deps.ManifestSection{
		"database":  &deps.Dependency{Repository: "https://github.com/okteto/database"},
		"messaging": &deps.Dependency{Repository: "https://github.com/okteto/messaging"},
	}
	runner := newFakeDependencyRunner(section)
	s := newDependencyScheduler(section, runner)

	s.Start(context.Background())
	requireStarted(t, runner, "database", "messaging")
	runner.finish("messaging", errors.New("boom"))
	runner.finish("database", errors.New("timeout"))

	require.EqualError(t, requireWaitResult(t, s), "dependency 'database' failed: timeout\ndependency 'messaging' failed: boom")
}

func TestDependencyScheduler_StopPreventsNewDependencies(t *testing.T) {
	section := specExampleSection()
	runner := newFakeDependencyRunner(section)
	s := newDependencyScheduler(section, runner)

	s.Start(context.Background())
	requireStarted(t, runner, "database", "messaging")

	s.Stop()
	runner.finish("database", nil)
	runner.finish("messaging", nil)
	err := requireWaitResult(t, s)
	requireNoMoreStarted(t, runner)

	require.EqualError(t, err, "dependencies not deployed because the deploy was stopped: analytics, backend")
}

func TestDependencyScheduler_StopBeforeStartStartsNothing(t *testing.T) {
	section := specExampleSection()
	runner := newFakeDependencyRunner(section)
	s := newDependencyScheduler(section, runner)

	s.Stop()
	s.Start(context.Background())
	err := requireWaitResult(t, s)
	requireNoMoreStarted(t, runner)

	require.EqualError(t, err, "dependencies not deployed because the deploy was stopped: analytics, backend, database, messaging")
}

func TestDependencyScheduler_StopBeforeMainReady(t *testing.T) {
	section := specExampleSection()
	runner := newFakeDependencyRunner(section)
	s := newDependencyScheduler(section, runner)

	s.Start(context.Background())
	mainReady := waitUntilMainReadyAsync(s)
	requireStarted(t, runner, "database", "messaging")

	s.Stop()
	runner.finish("database", nil)
	runner.finish("messaging", nil)

	require.ErrorIs(t, requireMainReadyResult(t, mainReady), errDependenciesStopped)
	require.Error(t, requireWaitResult(t, s))
}

func TestDependencyScheduler_FailureOfNotAwaitedDependencyDoesNotBlockMain(t *testing.T) {
	section := deps.ManifestSection{
		"database": &deps.Dependency{Repository: "https://github.com/okteto/database", Wait: true},
		"reports":  &deps.Dependency{Repository: "https://github.com/okteto/reports"},
	}
	runner := newFakeDependencyRunner(section)
	s := newDependencyScheduler(section, runner)

	s.Start(context.Background())
	mainReady := waitUntilMainReadyAsync(s)
	requireStarted(t, runner, "database", "reports")

	runner.finish("reports", errors.New("boom"))
	requireMainReadyPending(t, mainReady)

	runner.finish("database", nil)
	require.NoError(t, requireMainReadyResult(t, mainReady))
	// the deploy fails once everything has finished
	require.EqualError(t, requireWaitResult(t, s), "dependency 'reports' failed: boom")
}

func TestDependencyScheduler_MainPredecessorsKeepStartingAfterANotAwaitedFailure(t *testing.T) {
	section := deps.ManifestSection{
		"database":  &deps.Dependency{Repository: "https://github.com/okteto/database"},
		"messaging": &deps.Dependency{Repository: "https://github.com/okteto/messaging"},
		"backend": &deps.Dependency{
			Repository: "https://github.com/okteto/backend",
			DependsOn:  deps.DependsOn{"database", "messaging"},
			Wait:       true,
		},
		"reports": &deps.Dependency{Repository: "https://github.com/okteto/reports"},
	}
	runner := newFakeDependencyRunner(section)
	s := newDependencyScheduler(section, runner)

	s.Start(context.Background())
	mainReady := waitUntilMainReadyAsync(s)
	requireStarted(t, runner, "database", "messaging", "reports")
	runner.finish("reports", errors.New("boom"))
	runner.finish("database", nil)
	runner.finish("messaging", nil)

	requireStarted(t, runner, "backend")
	runner.finish("backend", nil)
	require.NoError(t, requireMainReadyResult(t, mainReady))
	require.EqualError(t, requireWaitResult(t, s), "dependency 'reports' failed: boom")
}

func TestDependencyScheduler_OnlyMainPredecessorsStartAfterAFailure(t *testing.T) {
	s := newDependencyScheduler(specExampleSection(), nil)

	require.True(t, s.startsAfterFailures("analytics", false))
	require.False(t, s.startsAfterFailures("analytics", true))
	require.True(t, s.startsAfterFailures("backend", true))
}

func TestDependencyScheduler_Diamond(t *testing.T) {
	section := deps.ManifestSection{
		"a": &deps.Dependency{Repository: "https://github.com/okteto/a"},
		"b": &deps.Dependency{Repository: "https://github.com/okteto/b", DependsOn: deps.DependsOn{"a"}},
		"c": &deps.Dependency{Repository: "https://github.com/okteto/c", DependsOn: deps.DependsOn{"a"}},
		"d": &deps.Dependency{Repository: "https://github.com/okteto/d", DependsOn: deps.DependsOn{"b", "c"}, Wait: true},
	}
	runner := newFakeDependencyRunner(section)
	s := newDependencyScheduler(section, runner)

	s.Start(context.Background())
	mainReady := waitUntilMainReadyAsync(s)
	requireStarted(t, runner, "a")
	runner.finish("a", nil)
	requireStarted(t, runner, "b", "c")
	runner.finish("b", nil)
	runner.finish("c", nil)
	requireStarted(t, runner, "d")
	runner.finish("d", nil)

	require.NoError(t, requireMainReadyResult(t, mainReady))
	require.NoError(t, requireWaitResult(t, s))
	requireNoMoreStarted(t, runner)
	require.Less(t, runner.indexOf(t, "end:b"), runner.indexOf(t, "start:d"))
	require.Less(t, runner.indexOf(t, "end:c"), runner.indexOf(t, "start:d"))
}

func TestDependencyScheduler_StartTwiceIsNoop(t *testing.T) {
	section := deps.ManifestSection{
		"database": &deps.Dependency{Repository: "https://github.com/okteto/database"},
	}
	runner := newFakeDependencyRunner(section)
	s := newDependencyScheduler(section, runner)

	s.Start(context.Background())
	s.Start(context.Background())
	requireStarted(t, runner, "database")
	runner.finish("database", nil)

	require.NoError(t, requireWaitResult(t, s))
	requireNoMoreStarted(t, runner)
}

func TestDependencyScheduler_CancelledKeepsPreviousFailures(t *testing.T) {
	section := deps.ManifestSection{
		"database":  &deps.Dependency{Repository: "https://github.com/okteto/database"},
		"messaging": &deps.Dependency{Repository: "https://github.com/okteto/messaging"},
	}
	runner := newFakeDependencyRunner(section)
	s := newDependencyScheduler(section, runner)
	ctx, cancel := context.WithCancel(context.Background())

	s.Start(ctx)
	requireStarted(t, runner, "database", "messaging")
	runner.finish("messaging", errors.New("boom"))

	cancel()
	err := requireWaitResult(t, s)

	require.ErrorIs(t, err, oktetoErrors.ErrIntSig)
	require.ErrorContains(t, err, "dependency 'messaging' failed: boom")
}

func TestDependencyScheduler_InterruptedDependency(t *testing.T) {
	section := specExampleSection()
	runner := newFakeDependencyRunner(section)
	s := newDependencyScheduler(section, runner)

	s.Start(context.Background())
	mainReady := waitUntilMainReadyAsync(s)
	requireStarted(t, runner, "database", "messaging")

	// the pipeline deployer handles Ctrl+C on its own and returns ErrIntSig without a cancelled context
	runner.finish("database", oktetoErrors.ErrIntSig)
	require.ErrorIs(t, requireMainReadyResult(t, mainReady), oktetoErrors.ErrIntSig)

	runner.finish("messaging", nil)
	require.ErrorIs(t, requireWaitResult(t, s), oktetoErrors.ErrIntSig)
	requireNoMoreStarted(t, runner)
}

func TestDependencyScheduler_Cancelled(t *testing.T) {
	section := specExampleSection()
	runner := newFakeDependencyRunner(section)
	s := newDependencyScheduler(section, runner)
	ctx, cancel := context.WithCancel(context.Background())

	s.Start(ctx)
	mainReady := waitUntilMainReadyAsync(s)
	requireStarted(t, runner, "database", "messaging")

	cancel()

	require.ErrorIs(t, requireMainReadyResult(t, mainReady), oktetoErrors.ErrIntSig)
	require.ErrorIs(t, requireWaitResult(t, s), oktetoErrors.ErrIntSig)
	requireNoMoreStarted(t, runner)
}

func runAllDependencies(t *testing.T, runner *pipelineDependencyRunner, section deps.ManifestSection) {
	t.Helper()
	for _, name := range sortedDependencyNames(section) {
		require.NoError(t, runner.Run(context.Background(), name, section[name]))
	}
}

// noDependencyEnvVars is the deployedDependencyEnvVarsGetter of dependencies that generate no env vars
func noDependencyEnvVars(context.Context, string, string) (map[string]string, error) {
	return map[string]string{}, nil
}

// fakeDependencyEnvVars returns the env vars of each dependency and records the dependencies asked for
type fakeDependencyEnvVars struct {
	envs  map[string]map[string]string
	asked []string
	mu    sync.Mutex
}

func (f *fakeDependencyEnvVars) get(_ context.Context, name, _ string) (map[string]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, name)
	return f.envs[name], nil
}

func TestPipelineDependencyRunnerOptions(t *testing.T) {
	section := specExampleSection()
	deployer := &capturingPipelineDeployer{}
	runner := newPipelineDependencyRunner(deployer, noDependencyEnvVars, section, &Options{Timeout: 5 * time.Minute}, "test", "parent-id")

	runAllDependencies(t, runner, section)

	backend := deployer.captured["backend"]
	require.True(t, backend.Wait)
	require.True(t, backend.IsDependency)
	require.True(t, backend.SkipIfExists)
	require.True(t, backend.PrefixLogs)
	require.Equal(t, "test", backend.Namespace)
	require.Equal(t, "parent-id", backend.ParentWorkflowID)
	require.Equal(t, 5*time.Minute, backend.Timeout)
	require.Contains(t, backend.Variables, "OKTETO_ORIGIN=okteto-deploy")
}

func TestPipelineDependencyRunnerChecksResourcesOnlyWhenNeeded(t *testing.T) {
	section := specExampleSection()
	deployer := &capturingPipelineDeployer{}
	runner := newPipelineDependencyRunner(deployer, noDependencyEnvVars, section, &Options{}, "test", "parent-id")

	runAllDependencies(t, runner, section)

	// predecessors of the main deploy or of another dependency need their resources running
	require.False(t, deployer.captured["database"].SkipResourcesCheck)
	require.False(t, deployer.captured["messaging"].SkipResourcesCheck)
	require.False(t, deployer.captured["backend"].SkipResourcesCheck)
	// wait: false and nobody depends on it: only waits for the pipeline to finish
	require.True(t, deployer.captured["analytics"].SkipResourcesCheck)
}

func TestPipelineDependencyRunnerChecksEveryResourceWithWaitFlag(t *testing.T) {
	section := specExampleSection()
	deployer := &capturingPipelineDeployer{}
	runner := newPipelineDependencyRunner(deployer, noDependencyEnvVars, section, &Options{Wait: true}, "test", "parent-id")

	runAllDependencies(t, runner, section)

	require.False(t, deployer.captured["analytics"].SkipResourcesCheck)
}

func TestPipelineDependencyRunnerRedeploysWithDependenciesFlag(t *testing.T) {
	section := specExampleSection()
	deployer := &capturingPipelineDeployer{}
	runner := newPipelineDependencyRunner(deployer, noDependencyEnvVars, section, &Options{Dependencies: true}, "test", "parent-id")

	runAllDependencies(t, runner, section)

	require.False(t, deployer.captured["database"].SkipIfExists)
}

func TestPipelineDependencyRunnerSingleDependencyKeepsCurrentOutput(t *testing.T) {
	section := deps.ManifestSection{
		"database": &deps.Dependency{Repository: "https://github.com/okteto/database", Wait: true},
	}
	deployer := &capturingPipelineDeployer{}
	runner := newPipelineDependencyRunner(deployer, noDependencyEnvVars, section, &Options{}, "test", "parent-id")

	runAllDependencies(t, runner, section)

	require.False(t, deployer.captured["database"].PrefixLogs)
}

func TestPipelineDependencyRunnerDoesNotModifyTheManifest(t *testing.T) {
	dep := &deps.Dependency{
		Repository: "https://github.com/okteto/database",
		Variables:  env.Environment{{Name: "SIZE", Value: "small"}},
	}
	section := deps.ManifestSection{"database": dep}
	runner := newPipelineDependencyRunner(&capturingPipelineDeployer{}, noDependencyEnvVars, section, &Options{}, "test", "parent-id")

	runAllDependencies(t, runner, section)

	require.Equal(t, env.Environment{{Name: "SIZE", Value: "small"}}, dep.Variables)
}

func TestPipelineDependencyRunnerRejectsReservedVariables(t *testing.T) {
	dep := &deps.Dependency{
		Repository: "https://github.com/okteto/database",
		Variables:  env.Environment{{Name: "OKTETO_CONTEXT", Value: "other"}},
	}
	deployer := &capturingPipelineDeployer{}
	runner := newPipelineDependencyRunner(deployer, noDependencyEnvVars, deps.ManifestSection{"database": dep}, &Options{}, "test", "parent-id")

	err := runner.Run(context.Background(), "database", dep)

	require.ErrorIs(t, err, validator.ErrReservedVariableName)
	require.Empty(t, deployer.captured)
}

func TestPipelineDependencyRunnerSingleNotAwaitedDependencyIsPrefixed(t *testing.T) {
	// the main deploy doesn't wait for it: their logs are interleaved
	section := deps.ManifestSection{
		"reports": &deps.Dependency{Repository: "https://github.com/okteto/reports"},
	}
	deployer := &capturingPipelineDeployer{}
	runner := newPipelineDependencyRunner(deployer, noDependencyEnvVars, section, &Options{}, "test", "parent-id")

	runAllDependencies(t, runner, section)

	require.True(t, deployer.captured["reports"].PrefixLogs)
}

func TestPipelineDependencyRunnerExpandsVariablesWithTheStartEnvironment(t *testing.T) {
	t.Setenv("DEPS_TEST_BRANCH", "at-start")
	dep := &deps.Dependency{Repository: "https://github.com/okteto/database", Branch: "${DEPS_TEST_BRANCH}"}
	section := deps.ManifestSection{"database": dep}
	deployer := &capturingPipelineDeployer{}
	runner := newPipelineDependencyRunner(deployer, noDependencyEnvVars, section, &Options{}, "test", "parent-id")

	// the main deploy exports env vars while the dependencies are running
	t.Setenv("DEPS_TEST_BRANCH", "exported-later")
	runAllDependencies(t, runner, section)

	require.Equal(t, "at-start", deployer.captured["database"].Branch)
}

// chainSection returns a chain of dependencies: frontend depends on backend, which depends on database
func chainSection() deps.ManifestSection {
	return deps.ManifestSection{
		"database": &deps.Dependency{Repository: "https://github.com/okteto/database"},
		"backend": &deps.Dependency{
			Repository: "https://github.com/okteto/backend",
			DependsOn:  deps.DependsOn{"database"},
			Variables:  env.Environment{{Name: "DB_HOST", Value: "${OKTETO_DEPENDENCY_DATABASE_VARIABLE_HOST}"}},
		},
		"frontend": &deps.Dependency{
			Repository: "https://github.com/okteto/frontend",
			DependsOn:  deps.DependsOn{"backend"},
			Variables: env.Environment{
				{Name: "API_URL", Value: "${OKTETO_DEPENDENCY_BACKEND_VARIABLE_URL}"},
				{Name: "DB_HOST", Value: "${OKTETO_DEPENDENCY_DATABASE_VARIABLE_HOST}"},
			},
		},
	}
}

func chainEnvVars() *fakeDependencyEnvVars {
	return &fakeDependencyEnvVars{envs: map[string]map[string]string{
		"database": {"OKTETO_DEPENDENCY_DATABASE_VARIABLE_HOST": "db.svc"},
		"backend":  {"OKTETO_DEPENDENCY_BACKEND_VARIABLE_URL": "https://backend"},
	}}
}

func TestPipelineDependencyRunnerExpandsVariablesWithTheDependsOnEnvVars(t *testing.T) {
	section := chainSection()
	deployer := &capturingPipelineDeployer{}
	runner := newPipelineDependencyRunner(deployer, chainEnvVars().get, section, &Options{}, "test", "parent-id")

	runAllDependencies(t, runner, section)

	require.Contains(t, deployer.captured["backend"].Variables, "DB_HOST=db.svc")
	require.Contains(t, deployer.captured["frontend"].Variables, "API_URL=https://backend")
}

func TestPipelineDependencyRunnerLeavesOutThePredecessorsOfDependsOn(t *testing.T) {
	section := chainSection()
	envVars := chainEnvVars()
	deployer := &capturingPipelineDeployer{}
	runner := newPipelineDependencyRunner(deployer, envVars.get, section, &Options{}, "test", "parent-id")

	require.NoError(t, runner.Run(context.Background(), "frontend", section["frontend"]))

	require.Equal(t, []string{"backend"}, envVars.asked)
	// frontend doesn't declare database, so its env vars are not expanded
	require.Contains(t, deployer.captured["frontend"].Variables, "DB_HOST=${OKTETO_DEPENDENCY_DATABASE_VARIABLE_HOST}")
}

func TestPipelineDependencyRunnerFailsWhenTheDependsOnEnvVarsCannotBeRead(t *testing.T) {
	section := specExampleSection()
	deployer := &capturingPipelineDeployer{}
	envVarsOf := func(context.Context, string, string) (map[string]string, error) {
		return nil, assert.AnError
	}
	runner := newPipelineDependencyRunner(deployer, envVarsOf, section, &Options{}, "test", "parent-id")

	err := runner.Run(context.Background(), "analytics", section["analytics"])

	require.ErrorIs(t, err, assert.AnError)
	require.Empty(t, deployer.captured)
}

func TestPipelineDependencyRunnerWaitsAtLeastTenMinutesForNotAwaitedDependencies(t *testing.T) {
	section := specExampleSection()
	deployer := &capturingPipelineDeployer{}
	runner := newPipelineDependencyRunner(deployer, noDependencyEnvVars, section, &Options{Timeout: 5 * time.Minute}, "test", "parent-id")

	runAllDependencies(t, runner, section)

	// the main deploy doesn't wait for analytics
	require.Equal(t, 10*time.Minute, deployer.captured["analytics"].Timeout)
	require.Equal(t, 5*time.Minute, deployer.captured["database"].Timeout)
}

func TestPipelineDependencyRunnerUsesALongerTimeoutFlagForNotAwaitedDependencies(t *testing.T) {
	section := specExampleSection()
	deployer := &capturingPipelineDeployer{}
	runner := newPipelineDependencyRunner(deployer, noDependencyEnvVars, section, &Options{Timeout: 20 * time.Minute}, "test", "parent-id")

	runAllDependencies(t, runner, section)

	require.Equal(t, 20*time.Minute, deployer.captured["analytics"].Timeout)
}

func TestPipelineDependencyRunnerUsesTheDependencyTimeout(t *testing.T) {
	section := specExampleSection()
	section["analytics"].Timeout = 2 * time.Minute
	deployer := &capturingPipelineDeployer{}
	runner := newPipelineDependencyRunner(deployer, noDependencyEnvVars, section, &Options{Timeout: 5 * time.Minute}, "test", "parent-id")

	runAllDependencies(t, runner, section)

	require.Equal(t, 2*time.Minute, deployer.captured["analytics"].Timeout)
}
