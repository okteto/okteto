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
	"github.com/okteto/okteto/pkg/model"
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

// buildOneFn builds a single service with the given runner
type buildOneFn func(ctx context.Context, svc *buildTypes.BuildInfo, runner basic.Builder) error

// buildInParallel builds svcs at the same time on a pool of workers, each with its own
// BuildKit connector, showing their output through one multiplexer
func (ob *OktetoBuilder) buildInParallel(ctx context.Context, runner *buildCmd.OktetoBuilder, svcs []*buildTypes.BuildInfo, options *types.BuildOptions, buildOne buildOneFn) error {
	conn := runner.GetConnector()
	// Connect worker 0 before the shared display starts, so its queue status shows as usual
	if err := connectBuildKit(ctx, conn); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Report it like a sequential build does: as the failure of the first service
		first := svcs[0]
		first.SetBuildDuration(0, 0, false)
		first.Metadata().ErrorReason = err.Error()
		return fmt.Errorf("error building service '%s': %w", first.Name(), err)
	}
	defer conn.Stop()

	// Smart builds may warn through the builder's output while computing a hash: compute
	// them all before the shared display starts. Builds read them from the hasher's cache.
	ob.computeBuildHashes(svcs, options.Manifest)

	if options.EnableStages {
		ob.ioCtrl.SetStage("Building services")
	}
	names := make([]string, 0, len(svcs))
	for _, svc := range svcs {
		names = append(names, svc.Name())
	}
	mux, err := buildCmd.NewMultiplexer(ob.ioCtrl, options.OutputMode, names)
	if err != nil {
		return err
	}
	ob.ioCtrl.Logger().Infof("building %d services in parallel, at most %d at the same time", len(svcs), ob.buildStrategy.maxConcurrency())
	pool := runner.NewWorkerPool(ctx, ob.buildStrategy.maxConcurrency())

	buildErr := ob.buildStrategy.Build(ctx, svcs, inBatchDependencies(options.Manifest.Build, svcs), parallelBuildFn(pool, mux, buildOne))

	if err := mux.Close(); err != nil {
		// The images are built: a display problem must not fail the command
		ob.ioCtrl.Logger().Infof("could not display the build output: %s", err)
	}
	pool.Close()
	return buildErr
}

// computeBuildHashes computes the smart build hash of each service, as building it would
func (ob *OktetoBuilder) computeBuildHashes(svcs []*buildTypes.BuildInfo, manifest *model.Manifest) {
	if !ob.smartBuildCtrl.IsEnabled() {
		return
	}
	isStackManifest := manifest.Type == model.StackType
	for _, svc := range svcs {
		ob.smartBuildCtrl.GetBuildHash(ob.getBuildInfoWithoutVolumeMounts(manifest.Build[svc.Name()], isStackManifest), svc.Name())
	}
}

// connectBuildKit connects conn and waits until BuildKit is ready
func connectBuildKit(ctx context.Context, conn buildCmd.BuildkitConnector) error {
	if err := conn.Start(ctx); err != nil {
		return err
	}
	return conn.WaitUntilIsReady(ctx)
}

// buildSequentially builds svcs one after another with the builder's runner and output
func (ob *OktetoBuilder) buildSequentially(ctx context.Context, svcs []*buildTypes.BuildInfo, options *types.BuildOptions, buildOne buildOneFn) error {
	return sequentialBuildStrategy{}.Build(ctx, svcs, nil, func(ctx context.Context, svc *buildTypes.BuildInfo, _ int) func() error {
		return func() error {
			if options.EnableStages {
				ob.ioCtrl.SetStage(fmt.Sprintf("Building service %s", svc.Name()))
			}
			return buildOne(ctx, svc, ob.Builder)
		}
	})
}

// parallelBuildFn returns the build function of a parallel batch: when a service is
// prepared it reserves a worker from pool, so workers are handed out in the order the
// strategy starts services; the build waits for that worker, routes its output to the
// service's display in mux, builds the service with buildOne and gives the worker back
// reset to its defaults
func parallelBuildFn(pool *buildCmd.WorkerPool, mux *buildCmd.Multiplexer, buildOne buildOneFn) buildServiceFn {
	return func(ctx context.Context, svc *buildTypes.BuildInfo, inFlight int) func() error {
		ticket := pool.Reserve(inFlight)
		return func() error {
			w, err := ticket.Wait(ctx)
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
}
