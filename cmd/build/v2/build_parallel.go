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
	"fmt"

	"github.com/okteto/okteto/cmd/build/basic"
	buildTypes "github.com/okteto/okteto/cmd/build/v2/types"
	"github.com/okteto/okteto/pkg/build"
	buildCmd "github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/types"
)

// inBatchDependencies returns, for each service, the services of svcs it depends on.
// Cached dependencies already published their env vars, so they are not waited for.
func inBatchDependencies(manifest build.ManifestBuild, svcs []*buildTypes.BuildInfo) map[string][]string {
	inBatch := make(map[string]bool, len(svcs))
	for _, svc := range svcs {
		inBatch[svc.Name()] = true
	}
	deps := map[string][]string{}
	for _, svc := range svcs {
		info, ok := manifest[svc.Name()]
		if !ok {
			continue
		}
		for _, dep := range info.DependsOn {
			if inBatch[dep] {
				deps[svc.Name()] = append(deps[svc.Name()], dep)
			}
		}
	}
	return deps
}

// buildInParallel builds svcs at the same time on a pool of workers, each with its own
// BuildKit connector, showing their output through one multiplexer
func (ob *OktetoBuilder) buildInParallel(ctx context.Context, runner *buildCmd.OktetoBuilder, svcs []*buildTypes.BuildInfo, deps map[string][]string, options *types.BuildOptions) error {
	conn := runner.GetConnector()
	// Connect worker 0 before the shared display starts, so its queue status shows as
	// usual. A failure here is retried and reported by the build itself.
	if err := conn.Start(ctx); err != nil {
		ob.ioCtrl.Logger().Infof("could not connect to BuildKit before the parallel builds: %s", err)
	} else if err := conn.WaitUntilIsReady(ctx); err != nil {
		ob.ioCtrl.Logger().Infof("BuildKit not ready before the parallel builds: %s", err)
	}
	defer conn.Stop()

	names := make([]string, 0, len(svcs))
	for _, svc := range svcs {
		names = append(names, svc.Name())
	}
	mux, err := buildCmd.NewMultiplexer(ob.ioCtrl, options.OutputMode, names)
	if err != nil {
		return err
	}
	pool := runner.NewWorkerPool(ctx, ob.buildStrategy.maxConcurrency())

	if options.EnableStages {
		ob.ioCtrl.SetStage("Building services")
	}
	buildOne := func(ctx context.Context, svc *buildTypes.BuildInfo, runner basic.Builder) error {
		return ob.buildService(ctx, svc, options, runner)
	}
	buildErr := ob.buildStrategy.Build(ctx, svcs, deps, parallelBuildFn(pool, mux, buildOne))

	muxErr := mux.Close()
	pool.Close()
	if buildErr != nil {
		return buildErr
	}
	if muxErr != nil {
		return fmt.Errorf("could not display the build output: %w", muxErr)
	}
	return nil
}

// parallelBuildFn returns the build function of a parallel batch: it takes a worker from
// pool, routes its output to the service's display in mux, builds the service with
// buildOne and gives the worker back reset to its defaults
func parallelBuildFn(pool *buildCmd.WorkerPool, mux *buildCmd.Multiplexer, buildOne func(ctx context.Context, svc *buildTypes.BuildInfo, runner basic.Builder) error) buildServiceFn {
	return func(ctx context.Context, svc *buildTypes.BuildInfo, inFlight int) error {
		w, err := pool.Acquire(ctx, inFlight)
		if err != nil {
			return err
		}
		defer pool.Release(w)
		defer mux.Finish(svc.Name())
		w.SetSolveFn(mux.SolveFn(svc.Name()))
		defer w.SetSolveFn(nil)

		return buildOne(ctx, svc, basic.Builder{BuildRunner: w, IoCtrl: mux.IO(svc.Name())})
	}
}
