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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/depot/depot-go/build"
	cliv1 "github.com/depot/depot-go/proto/depot/cli/v1"
	"github.com/moby/buildkit/client"
	buildkitClient "github.com/moby/buildkit/client"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDepotMachine struct {
	err error
}

func (m *fakeDepotMachine) Release() error {
	return m.err
}

func (m *fakeDepotMachine) Connect(ctx context.Context) (*buildkitClient.Client, error) {
	return nil, m.err
}

func Test_isDepotEnabled(t *testing.T) {
	tests := []struct {
		depotProject string
		depotToken   string
		expected     bool
	}{
		{
			depotProject: "project1",
			depotToken:   "token1",
			expected:     true,
		},
		{
			depotProject: "",
			depotToken:   "token2",
			expected:     false,
		},
		{
			depotProject: "project3",
			depotToken:   "",
			expected:     false,
		},
		{
			depotProject: "",
			depotToken:   "",
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("depotProject=%s, depotToken=%s", tt.depotProject, tt.depotToken), func(t *testing.T) {
			res := isDepotEnabled(tt.depotProject, tt.depotToken)
			assert.Equal(t, tt.expected, res)
		})
	}
}
func Test_newDepotBuilder(t *testing.T) {
	projectId := "test-project"
	token := "test-token"
	okCtx := &okteto.ContextStateless{}
	ioCtrl := &io.Controller{}

	builder := newDepotBuilder(projectId, token, okCtx, ioCtrl)

	assert.NotNil(t, builder)
	assert.Equal(t, ioCtrl, builder.ioCtrl)
	assert.Equal(t, token, builder.token)
	assert.Equal(t, projectId, builder.project)
	assert.Equal(t, okCtx, builder.okCtx)
}

func Test_depotBuilder_release(_ *testing.T) {
	mockErr := errors.New("mock error")
	mockMachine := &fakeDepotMachine{
		err: mockErr,
	}
	mockIOCtrl := io.NewIOController()

	db := &depotBuilder{
		ioCtrl:  mockIOCtrl,
		machine: mockMachine,
	}

	build := build.Build{
		Finish: func(err error) {},
	}
	db.release(build)
}

func TestDepotRun(t *testing.T) {
	fakeFs := afero.NewMemMapFs()

	tests := []struct {
		newDepotBuildErr  error
		acquireMachineErr error
		machineConnectErr error
		expected          error
		name              string
	}{
		{
			name:             "error depot build",
			newDepotBuildErr: assert.AnError,
			expected:         assert.AnError,
		},
		{
			name:              "error acquire machine",
			acquireMachineErr: assert.AnError,
			expected:          assert.AnError,
		},
		{
			name:              "error acquire machine",
			machineConnectErr: assert.AnError,
			expected:          assert.AnError,
		},
		{
			name:     "dockerfile does not exist",
			expected: os.ErrNotExist,
		},
		{
			name: "successful build",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var tempDir string
			if tt.expected == nil {
				tempDir = t.TempDir()
				filePath := filepath.Join(tempDir, "Dockerfile")
				err := afero.WriteFile(fakeFs, filePath, []byte("FROM scratch"), 0600)
				require.NoError(t, err)
			}

			db := &depotBuilder{
				ioCtrl: io.NewIOController(),
				newDepotBuild: func(context.Context, *cliv1.CreateBuildRequest, string) (build.Build, error) {
					return build.Build{
						Finish: func(err error) {},
					}, tt.newDepotBuildErr
				},
				acquireMachine: func(context.Context, string, string, string) (depotMachineConnector, error) {
					return &fakeDepotMachine{
						err: tt.machineConnectErr,
					}, tt.acquireMachineErr
				},
				fs: fakeFs,
				okCtx: &okteto.ContextStateless{
					Store: &okteto.ContextStore{
						Contexts: map[string]*okteto.Context{
							"test": {
								IsOkteto: true,
							},
						},
						CurrentContext: "test",
					},
				},
			}

			opts := &types.BuildOptions{
				Path: tempDir,
				ExportCache: []string{
					"export-image",
					"another-export-image",
				},
				CacheFrom: []string{
					"from-image",
				},
				Target:  "testTarget",
				NoCache: true,
				ExtraHosts: []types.HostMap{
					{Hostname: "test", IP: "testIP"},
					{Hostname: fmt.Sprintf("kubernetes.%s", "subdomain"), IP: "testIP"},
				},
				BuildArgs: []string{"arg1=value1"},
				Tag:       "okteto.dev/test:okteto",
				DevTag:    "okteto.dev/test:okteto",
			}
			runAndHandle := func(ctx context.Context, c *client.Client, opt *client.SolveOpt, buildOptions *types.BuildOptions, okCtx OktetoContextInterface, ioCtrl *io.Controller) error {
				return nil
			}
			err := db.Run(context.Background(), opts, runAndHandle)
			assert.ErrorIs(t, err, tt.expected)
		})
	}
}
