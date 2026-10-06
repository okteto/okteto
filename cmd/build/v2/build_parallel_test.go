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
	"bytes"
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/moby/buildkit/client"
	"github.com/okteto/okteto/cmd/build/basic"
	buildTypes "github.com/okteto/okteto/cmd/build/v2/types"
	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/build"
	buildCmd "github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildWithDependsOnParallelStrategyFallsBack(t *testing.T) {
	dir, err := createDockerfile(t)
	require.NoError(t, err)
	registry := newFakeRegistry()
	bc := NewFakeBuilder(test.NewFakeOktetoBuilder(registry), registry, fakeConfig{isOkteto: true})
	bc.buildStrategy = parallelBuildStrategy{maxParallel: 4}
	manifest := &model.Manifest{
		Name: "test",
		Build: build.ManifestBuild{
			"a": &build.Info{Context: dir, Dockerfile: filepath.Join(dir, "Dockerfile"), Image: "okteto/a:test"},
			"b": &build.Info{Context: dir, Dockerfile: filepath.Join(dir, "Dockerfile"), Image: "okteto/b:test", DependsOn: []string{"a"}},
		},
	}

	err = bc.Build(context.Background(), &types.BuildOptions{Manifest: manifest})

	require.NoError(t, err)
	_, err = registry.GetImageTagWithDigest("okteto/a:test")
	require.NoError(t, err)
	_, err = registry.GetImageTagWithDigest("okteto/b:test")
	require.NoError(t, err)
}

func TestInBatchDependenciesSkipsCachedServices(t *testing.T) {
	manifest := build.ManifestBuild{
		"a":      &build.Info{},
		"b":      &build.Info{DependsOn: []string{"a", "cached"}},
		"cached": &build.Info{},
	}
	svcs := buildTypes.NewBuildInfos("manifest", "ns", "repo", []string{"a", "b"})

	deps := inBatchDependencies(manifest, svcs)

	require.Equal(t, map[string][]string{"b": {"a"}}, deps)
}

// parallelTestConnector is a connector that is always connected
type parallelTestConnector struct{}

func (parallelTestConnector) Start(context.Context) error            { return nil }
func (parallelTestConnector) WaitUntilIsReady(context.Context) error { return nil }
func (parallelTestConnector) Stop()                                  {}
func (parallelTestConnector) GetBuildkitClient(context.Context) (*client.Client, error) {
	return &client.Client{}, nil
}
func (parallelTestConnector) GetType() string { return "test" }

func newParallelTestBatch(t *testing.T, svcs []string) (*buildCmd.OktetoBuilder, *buildCmd.WorkerPool, *buildCmd.Multiplexer) {
	t.Helper()
	ioCtrl := io.NewIOController()
	runner := buildCmd.NewOktetoBuilder(&okteto.ContextStateless{Store: okteto.GetContextStore()}, afero.NewMemMapFs(), ioCtrl, parallelTestConnector{})
	pool := runner.NewWorkerPool(context.Background(), 4)
	t.Cleanup(pool.Close)
	mux, err := buildCmd.NewMultiplexer(ioCtrl, "plain", svcs)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, mux.Close()) })
	return runner, pool, mux
}

func TestParallelBuildFnBuildsWithServiceOutput(t *testing.T) {
	_, pool, mux := newParallelTestBatch(t, []string{"api"})
	var got basic.Builder
	buildFn := parallelBuildFn(pool, mux, func(_ context.Context, _ *buildTypes.BuildInfo, runner basic.Builder) error {
		got = runner
		return nil
	})

	err := buildFn(context.Background(), buildTypes.NewBuildInfos("m", "ns", "repo", []string{"api"})[0], 1)

	require.NoError(t, err)
	require.Same(t, mux.IO("api"), got.IoCtrl)
	require.IsType(t, &buildCmd.OktetoBuilder{}, got.BuildRunner)
	require.False(t, got.ShowBuildMessage)
}

func TestParallelBuildFnResetsWorker(t *testing.T) {
	runner, pool, mux := newParallelTestBatch(t, []string{"api"})
	buildFn := parallelBuildFn(pool, mux, func(context.Context, *buildTypes.BuildInfo, basic.Builder) error {
		return nil
	})

	err := buildFn(context.Background(), buildTypes.NewBuildInfos("m", "ns", "repo", []string{"api"})[0], 1)
	require.NoError(t, err)
	w, err := pool.Acquire(context.Background(), 1)

	require.NoError(t, err)
	require.Same(t, runner, w)
	require.True(t, buildCmd.UsesDefaultSolveFn(w))
}

func TestParallelBuildFnReturnsBuildError(t *testing.T) {
	_, pool, mux := newParallelTestBatch(t, []string{"api"})
	buildFn := parallelBuildFn(pool, mux, func(context.Context, *buildTypes.BuildInfo, basic.Builder) error {
		return assert.AnError
	})

	err := buildFn(context.Background(), buildTypes.NewBuildInfos("m", "ns", "repo", []string{"api"})[0], 1)

	require.ErrorIs(t, err, assert.AnError)
}

func TestBuildServiceRecordsCancelledBuilds(t *testing.T) {
	dir, err := createDockerfile(t)
	require.NoError(t, err)
	registry := newFakeRegistry()
	bc := NewFakeBuilder(test.NewFakeOktetoBuilder(registry, assert.AnError), registry, fakeConfig{isOkteto: true})
	manifest := &model.Manifest{
		Name:  "test",
		Build: build.ManifestBuild{"a": &build.Info{Context: dir, Dockerfile: filepath.Join(dir, "Dockerfile"), Image: "okteto/a:test"}},
	}
	svc := buildTypes.NewBuildInfos("test", "ns", "repo", []string{"a"})[0]
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = bc.buildService(ctx, svc, &types.BuildOptions{Manifest: manifest}, bc.Builder)

	require.Error(t, err)
	require.Equal(t, "cancelled", svc.Metadata().ErrorReason)
}

func TestBuildServiceImagesWarnsThroughRunnerOutput(t *testing.T) {
	dir, err := createDockerfile(t)
	require.NoError(t, err)
	t.Setenv("OKTETO_TEST_UNSET_SECRET", "")
	registry := newFakeRegistry()
	bc := NewFakeBuilder(test.NewFakeOktetoBuilder(registry), registry, fakeConfig{isOkteto: true})
	manifest := &model.Manifest{
		Name: "test",
		Build: build.ManifestBuild{"a": &build.Info{
			Context:    dir,
			Dockerfile: filepath.Join(dir, "Dockerfile"),
			Image:      "okteto/a:test",
			Secrets:    build.Secrets{"token": build.Secret{Env: "OKTETO_TEST_UNSET_SECRET"}},
		}},
	}
	var out bytes.Buffer
	runner := basic.Builder{BuildRunner: bc.Builder.BuildRunner, IoCtrl: bc.ioCtrl.WithOutput(&out, "Building service a")}

	_, err = bc.buildServiceImages(context.Background(), manifest, buildTypes.NewBuildInfos("test", "ns", "repo", []string{"a"})[0], &types.BuildOptions{}, runner)

	require.NoError(t, err)
	require.Contains(t, out.String(), `secret "token" uses env var "OKTETO_TEST_UNSET_SECRET" which is not set`)
}

// startErrConnector is a connector whose Start fails with err, or with the context error
// when ctx is done
type startErrConnector struct {
	parallelTestConnector
	err error
}

func (c startErrConnector) Start(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return c.err
}

// builtServices records the runner each service was built with
type builtServices struct {
	runners sync.Map
}

func (b *builtServices) build(_ context.Context, svc *buildTypes.BuildInfo, runner basic.Builder) error {
	b.runners.Store(svc.Name(), runner)
	return nil
}

func (b *builtServices) runner(t *testing.T, svc string) basic.Builder {
	t.Helper()
	runner, ok := b.runners.Load(svc)
	require.True(t, ok, "service %s was not built", svc)
	return runner.(basic.Builder)
}

func newParallelTestBuilder(t *testing.T) *OktetoBuilder {
	t.Helper()
	registry := newFakeRegistry()
	ob := NewFakeBuilder(test.NewFakeOktetoBuilder(registry), registry, fakeConfig{isOkteto: true})
	ob.buildStrategy = parallelBuildStrategy{maxParallel: 4}
	return ob
}

func newTestRunner(conn buildCmd.BuildkitConnector) *buildCmd.OktetoBuilder {
	return buildCmd.NewOktetoBuilder(&okteto.ContextStateless{Store: okteto.GetContextStore()}, afero.NewMemMapFs(), io.NewIOController(), conn)
}

func parallelTestOptions() *types.BuildOptions {
	return &types.BuildOptions{
		OutputMode: "plain",
		Manifest:   &model.Manifest{Build: build.ManifestBuild{"a": &build.Info{}, "b": &build.Info{}}},
	}
}

func TestBuildInParallelBuildsEveryServiceWithItsOwnOutput(t *testing.T) {
	ob := newParallelTestBuilder(t)
	built := &builtServices{}

	err := ob.buildInParallel(context.Background(), newTestRunner(parallelTestConnector{}), buildTypes.NewBuildInfos("m", "ns", "repo", []string{"a", "b"}), parallelTestOptions(), built.build)

	require.NoError(t, err)
	require.NotSame(t, ob.ioCtrl, built.runner(t, "a").IoCtrl)
	require.NotSame(t, ob.ioCtrl, built.runner(t, "b").IoCtrl)
}

func TestBuildInParallelReturnsBuildError(t *testing.T) {
	ob := newParallelTestBuilder(t)

	err := ob.buildInParallel(context.Background(), newTestRunner(parallelTestConnector{}), buildTypes.NewBuildInfos("m", "ns", "repo", []string{"a", "b"}), parallelTestOptions(), func(context.Context, *buildTypes.BuildInfo, basic.Builder) error {
		return assert.AnError
	})

	require.ErrorIs(t, err, assert.AnError)
}

func TestBuildInParallelFallsBackToSequentialWhenBuildKitIsNotReady(t *testing.T) {
	ob := newParallelTestBuilder(t)
	built := &builtServices{}

	err := ob.buildInParallel(context.Background(), newTestRunner(startErrConnector{err: assert.AnError}), buildTypes.NewBuildInfos("m", "ns", "repo", []string{"a", "b"}), parallelTestOptions(), built.build)

	require.NoError(t, err)
	require.Same(t, ob.ioCtrl, built.runner(t, "a").IoCtrl)
	require.Same(t, ob.ioCtrl, built.runner(t, "b").IoCtrl)
}

func TestBuildInParallelReturnsCancellationWhileConnecting(t *testing.T) {
	ob := newParallelTestBuilder(t)
	built := &builtServices{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := ob.buildInParallel(ctx, newTestRunner(startErrConnector{}), buildTypes.NewBuildInfos("m", "ns", "repo", []string{"a", "b"}), parallelTestOptions(), built.build)

	require.ErrorIs(t, err, context.Canceled)
	_, builtA := built.runners.Load("a")
	require.False(t, builtA)
}
