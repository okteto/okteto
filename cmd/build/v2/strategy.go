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

	buildTypes "github.com/okteto/okteto/cmd/build/v2/types"
	"github.com/okteto/okteto/pkg/env"
)

const (
	// ParallelBuildStrategyEnvVar enables building independent services at the same time.
	// Default false while the parallel strategy is validated; false keeps the sequential
	// builds of previous versions.
	ParallelBuildStrategyEnvVar = "OKTETO_BUILD_STRATEGY_PARALLEL"

	// MaxParallelBuildsEnvVar is the maximum number of builds running at the same time.
	// Values lower than 2 build sequentially.
	MaxParallelBuildsEnvVar = "OKTETO_BUILD_MAX_PARALLEL"

	defaultMaxParallelBuilds = 4
)

// buildServiceFn builds a single service. inFlight is the number of builds running at
// that moment, including this one.
type buildServiceFn func(ctx context.Context, svc *buildTypes.BuildInfo, inFlight int) error

// buildStrategy decides how the services that are not cached are built. deps lists, for
// each service, the services of svcs it depends on.
type buildStrategy interface {
	Build(ctx context.Context, svcs []*buildTypes.BuildInfo, deps map[string][]string, build buildServiceFn) error
	// maxConcurrency is the maximum number of builds the strategy runs at the same time
	maxConcurrency() int
}

// newBuildStrategy returns the build strategy configured through the environment
func newBuildStrategy() buildStrategy {
	if !env.LoadBooleanOrDefault(ParallelBuildStrategyEnvVar, false) {
		return sequentialBuildStrategy{}
	}
	maxParallel := env.LoadIntOrDefault(MaxParallelBuildsEnvVar, defaultMaxParallelBuilds)
	if maxParallel <= 1 {
		return sequentialBuildStrategy{}
	}
	return parallelBuildStrategy{maxParallel: maxParallel}
}
