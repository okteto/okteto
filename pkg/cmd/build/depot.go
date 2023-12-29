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
	"os"
	"time"

	"github.com/depot/depot-go/build"
	"github.com/depot/depot-go/machine"
	cliv1 "github.com/depot/depot-go/proto/depot/cli/v1"
	buildkitClient "github.com/moby/buildkit/client"
)

const (
	depotToken   = "DEPOT_TOKEN"
	depotProject = "DEPOT_PROJECT"
)

type depotBuilder struct {
	client  *buildkitClient.Client
	machine *machine.Machine
	build   build.Build
	err     error
}

func depotBuilderEnabled() bool {
	return os.Getenv(depotToken) != "" && os.Getenv(depotProject) != ""
}

func newDepotBuilder(ctx context.Context, tag string) (*depotBuilder, error) {
	token := os.Getenv(depotToken)
	project := os.Getenv(depotProject)

	// Register a new build.
	req := &cliv1.CreateBuildRequest{
		ProjectId: project,
		Options: []*cliv1.BuildOptions{
			{
				Command: cliv1.Command_COMMAND_BUILD,
				Tags:    []string{tag},
				Push:    true,
				Load:    true,
			},
		},
	}

	dp := &depotBuilder{}
	dp.build, dp.err = build.NewBuild(ctx, req, token)
	if dp.err != nil {
		return nil, dp.err
	}

	// Acquire a buildkit machine.
	dp.machine, dp.err = machine.Acquire(ctx, dp.build.ID, dp.build.Token, "amd64")
	if dp.err != nil {
		return nil, dp.err
	}

	// Check buildkitd readiness. When the buildkitd starts, it may take
	// quite a while to be ready to accept connections when it loads a large boltdb.
	connectCtx, cancelConnect := context.WithTimeout(ctx, 1*time.Second)
	defer cancelConnect()

	dp.client, dp.err = dp.machine.Connect(connectCtx)
	if dp.err != nil {
		return nil, dp.err
	}

	return dp, nil
}

func (db *depotBuilder) release() {
	db.build.Finish(db.err)
	// ignore error releasing depot's machine
	_ = db.machine.Release()
}
