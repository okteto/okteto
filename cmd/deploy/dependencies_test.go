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
	"testing/synctest"
	"time"

	pipelineCMD "github.com/okteto/okteto/cmd/pipeline"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/deps"
	"github.com/okteto/okteto/pkg/env"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/validator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
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

	requireStarted(t, runner, "database", "messaging")

	runner.finish("database", nil)
	requireStarted(t, runner, "analytics")

	runner.finish("messaging", nil)
	requireStarted(t, runner, "backend")

	runner.finish("backend", nil)
	runner.finish("analytics", nil)
	require.NoError(t, requireWaitResult(t, s))
	requireNoMoreStarted(t, runner)

	require.Less(t, runner.indexOf(t, "end:database"), runner.indexOf(t, "start:analytics"))
	require.Less(t, runner.indexOf(t, "end:database"), runner.indexOf(t, "start:backend"))
	require.Less(t, runner.indexOf(t, "end:messaging"), runner.indexOf(t, "start:backend"))
}

func TestDependencyScheduler_NoDependencies(t *testing.T) {
	section := deps.ManifestSection{}
	s := newDependencyScheduler(section, newFakeDependencyRunner(section))

	s.Start(context.Background())

	require.NoError(t, requireWaitResult(t, s))
}

func TestDependencyScheduler_AFailureStopsNewDependencies(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		section := specExampleSection()
		runner := newFakeDependencyRunner(section)
		s := newDependencyScheduler(section, runner)

		s.Start(context.Background())
		requireStarted(t, runner, "database", "messaging")

		runner.finish("messaging", errors.New("boom"))
		// the scheduler has processed the failure before database ends
		synctest.Wait()
		// database was already running, so it is allowed to finish, but nothing new starts
		runner.finish("database", nil)
		err := requireWaitResult(t, s)
		requireNoMoreStarted(t, runner)

		require.EqualError(t, err, "dependency 'messaging' failed: boom\ndependencies not deployed because of a previous failure: analytics, backend")
	})
}

func TestDependencyScheduler_ADependencyOfAFailedOneNeverStarts(t *testing.T) {
	section := deps.ManifestSection{
		"a": &deps.Dependency{Repository: "https://github.com/okteto/a", Wait: true},
		"b": &deps.Dependency{Repository: "https://github.com/okteto/b", DependsOn: deps.DependsOn{"a"}},
	}
	runner := newFakeDependencyRunner(section)
	s := newDependencyScheduler(section, runner)

	s.Start(context.Background())
	requireStarted(t, runner, "a")
	runner.finish("a", errors.New("boom"))
	err := requireWaitResult(t, s)
	requireNoMoreStarted(t, runner)

	require.EqualError(t, err, "dependency 'a' failed: boom\ndependencies not deployed because of a previous failure: b")
}

func TestDependencyScheduler_ReportsEveryFailure(t *testing.T) {
	section := deps.ManifestSection{
		"database":  &deps.Dependency{Repository: "https://github.com/okteto/database", Wait: true},
		"messaging": &deps.Dependency{Repository: "https://github.com/okteto/messaging", Wait: true},
	}
	runner := newFakeDependencyRunner(section)
	s := newDependencyScheduler(section, runner)

	s.Start(context.Background())
	requireStarted(t, runner, "database", "messaging")
	runner.finish("messaging", errors.New("boom"))
	runner.finish("database", errors.New("timeout"))

	require.EqualError(t, requireWaitResult(t, s), "dependency 'database' failed: timeout\ndependency 'messaging' failed: boom")
}

// diamondSection returns a diamond: b and c depend on a, and d, with wait: true, depends on both
func diamondSection() deps.ManifestSection {
	return deps.ManifestSection{
		"a": &deps.Dependency{Repository: "https://github.com/okteto/a"},
		"b": &deps.Dependency{Repository: "https://github.com/okteto/b", DependsOn: deps.DependsOn{"a"}},
		"c": &deps.Dependency{Repository: "https://github.com/okteto/c", DependsOn: deps.DependsOn{"a"}},
		"d": &deps.Dependency{Repository: "https://github.com/okteto/d", DependsOn: deps.DependsOn{"b", "c"}, Wait: true},
	}
}

func TestDependencyScheduler_Diamond(t *testing.T) {
	section := diamondSection()
	runner := newFakeDependencyRunner(section)
	s := newDependencyScheduler(section, runner)

	s.Start(context.Background())
	requireStarted(t, runner, "a")
	runner.finish("a", nil)
	requireStarted(t, runner, "b", "c")
	runner.finish("b", nil)
	runner.finish("c", nil)
	requireStarted(t, runner, "d")
	runner.finish("d", nil)

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
	requireStarted(t, runner, "database", "messaging")

	// the pipeline deployer handles Ctrl+C on its own and returns ErrIntSig without a cancelled context
	runner.finish("database", oktetoErrors.ErrIntSig)
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
	requireStarted(t, runner, "database", "messaging")

	cancel()

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

// newTestDependencyRunner returns a runner of the dependencies of section in the namespace "test"
func newTestDependencyRunner(deployer pipelineCMD.DeployerInterface, envVarsOf deployedDependencyEnvVarsGetter, section deps.ManifestSection, opts *Options) *pipelineDependencyRunner {
	return newPipelineDependencyRunner(deployer, envVarsOf, section, opts, "test", "parent-id")
}

func TestPipelineDependencyRunnerOptions(t *testing.T) {
	section := specExampleSection()
	deployer := &capturingPipelineDeployer{}
	runner := newTestDependencyRunner(deployer, noDependencyEnvVars, section, &Options{Timeout: 5 * time.Minute})

	runAllDependencies(t, runner, section)

	backend := deployer.captured["backend"]
	require.True(t, backend.Wait)
	require.True(t, backend.IsDependency)
	require.True(t, backend.SkipIfExists)
	require.Equal(t, "test", backend.Namespace)
	require.Equal(t, "parent-id", backend.ParentWorkflowID)
	require.Equal(t, 5*time.Minute, backend.Timeout)
	require.Contains(t, backend.Variables, "OKTETO_ORIGIN=okteto-deploy")
}

func TestPipelineDependencyRunnerOnlySchedulesTheDependenciesNothingNeeds(t *testing.T) {
	section := specExampleSection()
	deployer := &capturingPipelineDeployer{}
	runner := newTestDependencyRunner(deployer, noDependencyEnvVars, section, &Options{})

	runAllDependencies(t, runner, section)

	// neither the main deploy nor another dependency waits for analytics
	require.False(t, deployer.captured["analytics"].Wait)
	// database and messaging have wait: false, but backend needs them
	require.True(t, deployer.captured["database"].Wait)
	require.True(t, deployer.captured["messaging"].Wait)
	require.True(t, deployer.captured["backend"].Wait)
}

func TestPipelineDependencyRunnerFollowsTheSameRulesInsideADependency(t *testing.T) {
	t.Setenv(constants.OktetoIsDependencyEnvVar, "true")
	section := specExampleSection()
	deployer := &capturingPipelineDeployer{}
	runner := newTestDependencyRunner(deployer, noDependencyEnvVars, section, &Options{})

	runAllDependencies(t, runner, section)

	require.False(t, deployer.captured["analytics"].Wait)
	require.True(t, deployer.captured["database"].Wait)
	require.True(t, deployer.captured["messaging"].Wait)
	require.True(t, deployer.captured["backend"].Wait)
}

func TestPipelineDependencyRunnerWaitsForAWaitFalseDependencyAnotherOneNeeds(t *testing.T) {
	section := deps.ManifestSection{
		"a": &deps.Dependency{Repository: "https://github.com/okteto/a"},
		"b": &deps.Dependency{Repository: "https://github.com/okteto/b", DependsOn: deps.DependsOn{"a"}},
	}
	deployer := &capturingPipelineDeployer{}
	runner := newTestDependencyRunner(deployer, noDependencyEnvVars, section, &Options{})

	runAllDependencies(t, runner, section)

	require.True(t, deployer.captured["a"].Wait)
	require.False(t, deployer.captured["b"].Wait)
}

func TestPipelineDependencyRunnerOnlySchedulesTheDependenciesNothingNeedsWithWaitFlag(t *testing.T) {
	section := specExampleSection()
	deployer := &capturingPipelineDeployer{}
	runner := newTestDependencyRunner(deployer, noDependencyEnvVars, section, &Options{Wait: true})

	runAllDependencies(t, runner, section)

	require.False(t, deployer.captured["analytics"].Wait)
}

func TestPipelineDependencyRunnerRedeploysWithDependenciesFlag(t *testing.T) {
	section := specExampleSection()
	deployer := &capturingPipelineDeployer{}
	runner := newTestDependencyRunner(deployer, noDependencyEnvVars, section, &Options{Dependencies: true})

	runAllDependencies(t, runner, section)

	require.False(t, deployer.captured["database"].SkipIfExists)
}

func TestPipelineDependencyRunnerDoesNotModifyTheManifest(t *testing.T) {
	dep := &deps.Dependency{
		Repository: "https://github.com/okteto/database",
		Variables:  env.Environment{{Name: "SIZE", Value: "small"}},
	}
	section := deps.ManifestSection{"database": dep}
	runner := newTestDependencyRunner(&capturingPipelineDeployer{}, noDependencyEnvVars, section, &Options{})

	runAllDependencies(t, runner, section)

	require.Equal(t, env.Environment{{Name: "SIZE", Value: "small"}}, dep.Variables)
}

func TestPipelineDependencyRunnerRejectsReservedVariables(t *testing.T) {
	dep := &deps.Dependency{
		Repository: "https://github.com/okteto/database",
		Variables:  env.Environment{{Name: "OKTETO_CONTEXT", Value: "other"}},
	}
	section := deps.ManifestSection{"database": dep}
	deployer := &capturingPipelineDeployer{}
	runner := newTestDependencyRunner(deployer, noDependencyEnvVars, section, &Options{})
	printed := captureDependencyOutput(t, oktetoLog.JSONFormat)

	err := runner.Run(context.Background(), "database", dep)

	require.ErrorIs(t, err, validator.ErrReservedVariableName)
	require.Empty(t, deployer.captured)
	require.Equal(t, []string{"error: [database]  x  OKTETO_CONTEXT is reserved variable name"}, printed())
}

func TestPipelineDependencyRunnerPrintsTheErrorOfTheDeploy(t *testing.T) {
	section := deps.ManifestSection{"database": &deps.Dependency{Repository: "https://github.com/okteto/database", Wait: true}}
	deployer := &capturingPipelineDeployer{errs: map[string]error{"database": errors.New("repository 'database' deployed with errors")}}
	runner := newTestDependencyRunner(deployer, noDependencyEnvVars, section, &Options{})
	printed := captureDependencyOutput(t, oktetoLog.JSONFormat)

	err := runner.Run(context.Background(), "database", section["database"])

	require.Error(t, err)
	require.Equal(t, []string{"error: [database]  x  repository 'database' deployed with errors"}, printed())
}

func TestPipelineDependencyRunnerExpandsVariablesWithTheStartEnvironment(t *testing.T) {
	t.Setenv("DEPS_TEST_BRANCH", "at-start")
	dep := &deps.Dependency{Repository: "https://github.com/okteto/database", Branch: "${DEPS_TEST_BRANCH}"}
	section := deps.ManifestSection{"database": dep}
	deployer := &capturingPipelineDeployer{}
	runner := newTestDependencyRunner(deployer, noDependencyEnvVars, section, &Options{})

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
	runner := newTestDependencyRunner(deployer, chainEnvVars().get, section, &Options{})

	runAllDependencies(t, runner, section)

	require.Contains(t, deployer.captured["backend"].Variables, "DB_HOST=db.svc")
	require.Contains(t, deployer.captured["frontend"].Variables, "API_URL=https://backend")
}

func TestPipelineDependencyRunnerLeavesOutThePredecessorsOfDependsOn(t *testing.T) {
	section := chainSection()
	envVars := chainEnvVars()
	deployer := &capturingPipelineDeployer{}
	runner := newTestDependencyRunner(deployer, envVars.get, section, &Options{})

	require.NoError(t, runner.Run(context.Background(), "frontend", section["frontend"]))

	require.Equal(t, []string{"backend"}, envVars.asked)
	// frontend doesn't declare database, so its env vars are undefined, as any other undefined env var
	require.Contains(t, deployer.captured["frontend"].Variables, "DB_HOST=")
}

func TestPipelineDependencyRunnerFailsWhenTheDependsOnEnvVarsCannotBeRead(t *testing.T) {
	section := specExampleSection()
	deployer := &capturingPipelineDeployer{}
	envVarsOf := func(context.Context, string, string) (map[string]string, error) {
		return nil, assert.AnError
	}
	runner := newTestDependencyRunner(deployer, envVarsOf, section, &Options{})

	err := runner.Run(context.Background(), "analytics", section["analytics"])

	require.ErrorIs(t, err, assert.AnError)
	require.Empty(t, deployer.captured)
}

func TestPipelineDependencyRunnerUsesTheTimeoutFlagForEveryDependency(t *testing.T) {
	section := specExampleSection()
	deployer := &capturingPipelineDeployer{}
	runner := newTestDependencyRunner(deployer, noDependencyEnvVars, section, &Options{Timeout: 5 * time.Minute})

	runAllDependencies(t, runner, section)

	// analytics is only scheduled: it doesn't need a longer timeout
	require.Equal(t, 5*time.Minute, deployer.captured["analytics"].Timeout)
	require.Equal(t, 5*time.Minute, deployer.captured["database"].Timeout)
}

func TestPipelineDependencyRunnerUsesTheDependencyTimeout(t *testing.T) {
	section := specExampleSection()
	section["analytics"].Timeout = 2 * time.Minute
	deployer := &capturingPipelineDeployer{}
	runner := newTestDependencyRunner(deployer, noDependencyEnvVars, section, &Options{Timeout: 5 * time.Minute})

	runAllDependencies(t, runner, section)

	require.Equal(t, 2*time.Minute, deployer.captured["analytics"].Timeout)
}

func TestPipelineDependencyRunnerExpandsDependsOnEnvVarsWrittenInTheManifest(t *testing.T) {
	manifest := []byte(`
database:
  repository: https://github.com/okteto/database
backend:
  repository: https://github.com/okteto/backend
  depends_on: database
  variables:
    DB_HOST: ${OKTETO_DEPENDENCY_DATABASE_VARIABLE_HOST}
`)
	var section deps.ManifestSection
	require.NoError(t, yaml.Unmarshal(manifest, &section))
	deployer := &capturingPipelineDeployer{}
	runner := newTestDependencyRunner(deployer, chainEnvVars().get, section, &Options{})

	runAllDependencies(t, runner, section)

	require.Contains(t, deployer.captured["backend"].Variables, "DB_HOST=db.svc")
}

func TestPipelineDependencyRunnerExpandsTheDependsOnEnvVarsOfAScheduledDependency(t *testing.T) {
	section := deps.ManifestSection{
		"database": &deps.Dependency{Repository: "https://github.com/okteto/database"},
		"reports": &deps.Dependency{
			Repository: "https://github.com/okteto/reports",
			DependsOn:  deps.DependsOn{"database"},
			Variables:  env.Environment{{Name: "DB_HOST", Value: "${OKTETO_DEPENDENCY_DATABASE_VARIABLE_HOST}"}},
		},
	}
	deployer := &capturingPipelineDeployer{}
	runner := newTestDependencyRunner(deployer, chainEnvVars().get, section, &Options{})

	runAllDependencies(t, runner, section)

	require.False(t, deployer.captured["reports"].Wait)
	require.Contains(t, deployer.captured["reports"].Variables, "DB_HOST=db.svc")
}
