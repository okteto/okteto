// Copyright 2023 The Okteto Authors
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

package buildkit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moby/buildkit/client"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/env"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
)

const (
	// defaultMaxAttempts is the default number of attempts to try to do a build
	defaultMaxAttempts = 3

	// MaxRetriesForBuildkitTransientErrorsEnvVar is the environment variable to set the number of retries to wait for buildkit to be available
	MaxRetriesForBuildkitTransientErrorsEnvVar = "OKTETO_BUILDKIT_MAX_RETRIES_FOR_TRANSIENT_ERRORS"
)

type registryImageChecker interface {
	GetImageTagWithDigest(tag string) (string, error)
	IsOktetoRegistry(image string) bool
	IsGlobalRegistry(image string) bool
}

type buildkitWaiterInterface interface {
	WaitUntilIsUp(ctx context.Context) error
}

type buildkitClientFactory interface {
	GetBuildkitClient(ctx context.Context) (*client.Client, error)
}

type runnerMetadata struct {
	attempts int
	// TODO: use this to track the time it takes to solve a build instead of the full build time
	solveTime time.Duration
}

// SolveOptBuilderFactory is a function that creates a SolveOptBuilderInterface
type SolveOptBuilderFactory func(cf clientFactory, reg IsInOktetoRegistryChecker, okCtx OktetoContextInterface, fs afero.Fs, logger *io.Controller) (SolveOptBuilderInterface, error)

// Runner runs a build using buildkit
type Runner struct {
	connector                          buildkitConnector
	solveBuild                         SolveBuildFn
	registry                           registryImageChecker
	logger                             *io.Controller
	metadata                           *runnerMetadata
	maxAttemptsBuildkitTransientErrors int
	okCtx                              OktetoContextInterface
	fs                                 afero.Fs
	solveOptBuilderFactory             SolveOptBuilderFactory
}

// buildkitConnector is the interface for the buildkit connector
type buildkitConnector interface {
	// Start establishes the connection to the buildkit server
	Start(ctx context.Context) error
	// WaitUntilIsReady waits for the buildkit server to be ready
	WaitUntilIsReady(ctx context.Context) error
	// Stop closes the connection to the buildkit server
	Stop()
	// GetBuildkitClient returns the buildkit client
	GetBuildkitClient(ctx context.Context) (*client.Client, error)
}

type SolveOptBuilderInterface interface {
	Build(ctx context.Context, buildOptions *types.BuildOptions) (*client.SolveOpt, error)
}

// SolveBuildFn is a function that solves a build
type SolveBuildFn func(ctx context.Context, c *client.Client, opt *client.SolveOpt, progress string, ioCtrl *io.Controller) error

// defaultSolveOptBuilderFactory is the default factory that creates a SolveOptBuilder
func defaultSolveOptBuilderFactory(cf clientFactory, reg IsInOktetoRegistryChecker, okCtx OktetoContextInterface, fs afero.Fs, logger *io.Controller) (SolveOptBuilderInterface, error) {
	return NewSolveOptBuilder(cf, reg, okCtx, fs, logger)
}

// NewBuildkitRunner creates a new buildkit runner
func NewBuildkitRunner(connector buildkitConnector, registry registryImageChecker, solver SolveBuildFn, okCtx OktetoContextInterface, fs afero.Fs, logger *io.Controller) *Runner {
	return &Runner{
		connector:                          connector,
		solveBuild:                         solver,
		maxAttemptsBuildkitTransientErrors: env.LoadIntOrDefault(MaxRetriesForBuildkitTransientErrorsEnvVar, defaultMaxAttempts),
		logger:                             logger,
		registry:                           registry,
		metadata:                           &runnerMetadata{},
		okCtx:                              okCtx,
		fs:                                 fs,
		solveOptBuilderFactory:             defaultSolveOptBuilderFactory,
	}
}

// Run executes a build using buildkit
func (r *Runner) Run(ctx context.Context, buildOptions *types.BuildOptions, outputMode string) error {
	if err := r.connector.Start(ctx); err != nil {
		return err
	}

	defer r.connector.Stop()

	if err := r.connector.WaitUntilIsReady(ctx); err != nil {
		return err
	}

	optBuilder, err := r.solveOptBuilderFactory(r.connector, r.registry, r.okCtx, r.fs, r.logger)
	if err != nil {
		return err
	}

	opt, err := optBuilder.Build(ctx, buildOptions)
	if err != nil {
		return fmt.Errorf("failed to create build solver: %w", err)
	}
	tag := r.extractTagsFromOpt(opt)
	attempts := 0
	var solveTime time.Duration
	defer func() {
		r.metadata.attempts = attempts
		r.metadata.solveTime = solveTime
	}()

	var showBuildMessage bool
	switch buildOptions.OutputMode {
	case "test", "deploy", "destroy":
		showBuildMessage = false
	default:
		showBuildMessage = true
	}

	if showBuildMessage {
		builder := r.okCtx.GetCurrentBuilder()
		r.logger.Out().Infof("Building '%s' in %s...", buildOptions.Path, builder)
	}

	for {
		attempts++
		if attempts > 1 {
			r.logger.Logger().Infof("retrying build, attempt %d", attempts)
			r.logger.Out().Warning("BuildKit service connection failure. Retrying...")
		}
		if err := r.connector.Start(ctx); err != nil {
			r.logger.Logger().Infof("failed to start buildkit connector: %s", err)
			return fmt.Errorf("could not start buildkit connector: %w", err)
		}

		// if buildkit is not available for 10 minutes, we should fail
		if err := r.connector.WaitUntilIsReady(ctx); err != nil {
			r.logger.Logger().Infof("failed to wait for BuildKit service to be available: %s", err)
			return err
		}

		client, err := r.connector.GetBuildkitClient(ctx)
		if err != nil {
			r.logger.Logger().Infof("failed to get buildkit client: %s", err)
			if attempts >= r.maxAttemptsBuildkitTransientErrors {
				return ErrBuildConnecionFailed
			}
			continue
		}

		startSolverTime := time.Now()
		err = r.solveBuild(ctx, client, opt, outputMode, r.logger)
		solveTime = time.Since(startSolverTime)
		if err != nil {
			if IsRetryable(err) {
				r.connector.Stop()
				r.logger.Logger().Infof("retrying operation: %s", err)
				analytics.TrackBuildTransientError(true)
				if attempts >= r.maxAttemptsBuildkitTransientErrors {
					return ErrBuildConnecionFailed
				}
				continue
			}
			err = GetSolveErrorMessage(err)
			analytics.TrackBuildTransientError(false)
			return err
		}

		// Check if the image was pushed correctly to the registry
		if err := r.checkIfImageIsPushed(tag); err != nil {
			r.logger.Logger().Infof("failed to check if the image was pushed: %s", err)
			if attempts >= r.maxAttemptsBuildkitTransientErrors {
				return ErrBuildConnecionFailed
			}
			continue
		}
		// The image was built and pushed correctly
		return nil
	}
}

// extractTagsFromOpt extracts the tags from the solve options
func (r *Runner) extractTagsFromOpt(opt *client.SolveOpt) string {
	if opt == nil {
		return ""
	}
	for _, o := range opt.Exports {
		if o.Type == "image" && o.Attrs != nil && o.Attrs["push"] == "true" {
			return o.Attrs["name"]
		}
	}
	return ""
}

// checkIfImageIsPushed checks if the image was pushed correctly to the registry
func (r *Runner) checkIfImageIsPushed(tag string) error {
	if tag == "" {
		return nil
	}
	tags := strings.Split(tag, ",")
	for _, t := range tags {
		_, err := r.registry.GetImageTagWithDigest(t)
		if err != nil {
			return fmt.Errorf("failed to retrieve image tag '%s' from registry: %w", t, err)
		}
	}
	return nil
}
