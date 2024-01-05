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
	"time"

	"github.com/depot/depot-go/build"
	"github.com/depot/depot-go/machine"
	cliv1 "github.com/depot/depot-go/proto/depot/cli/v1"
	buildkitClient "github.com/moby/buildkit/client"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
)

const (
	depotTokenEnvVar   = "DEPOT_TOKEN"
	depotProjectEnvVar = "DEPOT_PROJECT"

	defaultPlatform = "amd64"
)

type depotMachineConnector interface {
	Connect(ctx context.Context) (*buildkitClient.Client, error)
	Release() error
}

type depotBuilder struct {
	token   string
	project string

	ioCtrl *io.IOController
	okCtx  OktetoContextInterface
	fs     afero.Fs

	newDepotBuild  func(ctx context.Context, req *cliv1.CreateBuildRequest, token string) (build.Build, error)
	acquireMachine func(ctx context.Context, buildId, token, platform string) (depotMachineConnector, error)

	machine depotMachineConnector
	err     error
}

// isDepotEnabled returns true if depot env vars are set
func isDepotEnabled(depotProject, depotToken string) bool {
	return depotToken != "" && depotProject != ""
}

// newDepotBuilder creates a new instance of DepotBuilder.
func newDepotBuilder(ctx context.Context, projectId, token string, okCtx OktetoContextInterface, ioCtrl *io.IOController) *depotBuilder {
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

	err := db.machine.Release()
	if err != nil {
		db.ioCtrl.Logger().Infof("failed to release depot's machine: %s", err)
	}
}

func (db *depotBuilder) Run(ctx context.Context, buildOptions *types.BuildOptions) error {
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
		return err
	}
	if buildOptions.Platform == "" {
		buildOptions.Platform = defaultPlatform
	}

	db.machine, err = db.acquireMachine(ctx, build.ID, build.Token, buildOptions.Platform)
	if err != nil {
		return err
	}

	client, err := db.getBuildkitClient(ctx, build, buildOptions.Platform)
	if err != nil {
		return err
	}

	if buildOptions.File != "" {
		buildOptions.File, err = GetDockerfile(buildOptions.File, db.okCtx)
		if err != nil {
			return err
		}
		defer db.fs.Remove(buildOptions.File)
	}

	// create a temp folder - this will be remove once the build has finished
	secretTempFolder, err := createSecretTempFolder()
	if err != nil {
		return err
	}
	defer db.fs.RemoveAll(secretTempFolder)

	opt, err := getSolveOpt(buildOptions, db.okCtx, secretTempFolder)
	if err != nil {
		return fmt.Errorf("failed to create build solver: %w", err)
	}

	db.ioCtrl.Logger().Info("Acquiring a depot's machine..")

	defer db.release(build)
	db.ioCtrl.Logger().Infof("building '%s' on depot's machine..", buildOptions.Tag)

	return runAndHandleBuild(ctx, client, opt, buildOptions, db.okCtx, db.ioCtrl)
}

// getBuildkitClient returns a buildkit client connected to the depot's machine.
func (db *depotBuilder) getBuildkitClient(ctx context.Context, build build.Build, platform string) (*buildkitClient.Client, error) {
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
