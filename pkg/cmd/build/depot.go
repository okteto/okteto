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

package build

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/depot/depot-go/build"
	"github.com/depot/depot-go/machine"
	cliv1 "github.com/depot/depot-go/proto/depot/cli/v1"
	"github.com/moby/buildkit/client"
	buildkitClient "github.com/moby/buildkit/client"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
)

const (
	DepotTokenEnvVar   = "OKTETO_DEPOT_TOKEN"
	DepotProjectEnvVar = "OKTETO_DEPOT_PROJECT_ID"

	defaultPlatform = "amd64"
)

type depotMachineConnector interface {
	Connect(ctx context.Context) (*buildkitClient.Client, error)
	Release() error
}

type depotBuilder struct {
	okCtx          OktetoContextInterface
	fs             afero.Fs
	machine        depotMachineConnector
	err            error
	ioCtrl         *io.Controller
	newDepotBuild  func(ctx context.Context, req *cliv1.CreateBuildRequest, token string) (build.Build, error)
	acquireMachine func(ctx context.Context, buildId, token, platform string) (depotMachineConnector, error)
	token          string
	project        string
	isRetry        bool
}

func IsDepotEnabled() bool {
	return os.Getenv(DepotTokenEnvVar) != "" && os.Getenv(DepotProjectEnvVar) != ""
}

// newDepotBuilder creates a new instance of DepotBuilder.
func newDepotBuilder(projectId, token string, okCtx OktetoContextInterface, ioCtrl *io.Controller) *depotBuilder {
	return &depotBuilder{
		ioCtrl:  ioCtrl,
		token:   token,
		project: projectId,
		okCtx:   okCtx,
		fs:      afero.NewOsFs(),

		newDepotBuild: build.NewBuild,
		acquireMachine: func(ctx context.Context, buildId, token, platform string) (depotMachineConnector, error) {
			return machine.Acquire(ctx, buildId, token, platform)
		},
	}
}

// release releases the depot's machine and finishes the build.
func (db *depotBuilder) release(build build.Build) {
	build.Finish(db.err)

	if db.machine == nil {
		return
	}
	err := db.machine.Release()
	if err != nil {
		db.ioCtrl.Logger().Infof("failed to release depot's machine: %s", err)
	}
}

type runAndHandleBuildFn func(ctx context.Context, c *client.Client, opt *client.SolveOpt, progress string, ioCtrl *io.Controller) error

func (db *depotBuilder) Run(ctx context.Context, buildOptions *types.BuildOptions, run runAndHandleBuildFn) error {
	db.ioCtrl.Logger().Info("building your image on depot's machine")

	// Register a new build.
	req := &cliv1.CreateBuildRequest{
		ProjectId: db.project,
		Options: []*cliv1.BuildOptions{
			{
				Command: cliv1.Command_COMMAND_BUILD,
				Tags:    []string{buildOptions.Tag},
				Push:    true,
				Load:    true,
			},
		},
	}

	build, err := db.newDepotBuild(ctx, req, db.token)
	if err != nil {
		return fmt.Errorf("depot build failed: %w", err)
	}
	if buildOptions.Platform == "" {
		buildOptions.Platform = defaultPlatform
	}

	platforms := strings.Split(buildOptions.Platform, ",")
	machinePlatform := buildOptions.Platform
	if len(platforms) > 1 {
		db.ioCtrl.Logger().Infof("[depot] multi-platform build detected: %s", buildOptions.Platform)
		machinePlatform = defaultPlatform
	}

	db.ioCtrl.Logger().Infof("[depot] connecting to %s machine", machinePlatform)
	db.machine, err = db.acquireMachine(ctx, build.ID, build.Token, machinePlatform)
	if err != nil {
		return err
	}

	defer db.release(build)

	client, err := db.getBuildkitClient(ctx)
	if err != nil {
		return err
	}

	if buildOptions.File != "" {
		buildOptions.File, err = GetDockerfile(buildOptions.File, db.okCtx)
		if err != nil {
			return err
		}
		defer func() {
			err := db.fs.Remove(buildOptions.File)
			if err != nil {
				db.ioCtrl.Logger().Infof("failed to remove file after build process: %s", err)
			}
		}()
	}

	// create a temp folder - this will be removed once the build has finished
	secretTempFolder, err := createSecretTempFolder()
	if err != nil {
		return err
	}
	defer func() {
		err := db.fs.RemoveAll(secretTempFolder)
		if err != nil {
			db.ioCtrl.Logger().Infof("failed to remove temp secrets folder after build process: %s", err)
		}
	}()

	opt, err := getSolveOpt(buildOptions, db.okCtx, secretTempFolder, db.fs)
	if err != nil {
		return fmt.Errorf("failed to create build solver: %w", err)
	}

	db.ioCtrl.Logger().Infof("[depot] build URL: %s", build.BuildURL)

	err = run(ctx, client, opt, buildOptions.OutputMode, db.ioCtrl)
	if err != nil {
		if shouldRetryBuild(err, buildOptions.Tag, db.okCtx) {
			db.ioCtrl.Logger().Infof("Failed to build image: %s", err.Error())
			db.ioCtrl.Logger().Infof("isRetry: %t", db.isRetry)
			if !db.isRetry {
				retryBuilder := newDepotBuilder(db.project, db.token, db.okCtx, db.ioCtrl)
				retryBuilder.isRetry = true
				err = retryBuilder.Run(ctx, buildOptions, run)
			}
		}
		err = getErrorMessage(err, buildOptions.Tag)
		return err
	}

	var tag string
	if buildOptions != nil {
		tag = buildOptions.Tag
		if buildOptions.Manifest != nil && buildOptions.Manifest.Deploy != nil {
			tag = buildOptions.Manifest.Deploy.Image
		}
	}
	err = getErrorMessage(err, tag)
	return err
}

// getBuildkitClient returns a buildkit client connected to the depot's machine.
func (db *depotBuilder) getBuildkitClient(ctx context.Context) (*buildkitClient.Client, error) {
	// Check buildkitd readiness. When the buildkitd starts, it may take
	// quite a while to be ready to accept connections when it loads a large boltdb.
	connectCtx, cancelConnect := context.WithTimeout(ctx, 1*time.Second)
	defer cancelConnect()

	client, err := db.machine.Connect(connectCtx)
	if err != nil {
		return nil, err
	}

	return client, nil
}
