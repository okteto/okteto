// Copyright 2022 The Okteto Authors
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

package pipeline

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/okteto/okteto/internal/test/client"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
)

func Test_getRepositoryURL(t *testing.T) {

	type remote struct {
		name string
		url  string
	}
	var tests = []struct {
		name        string
		expectError bool
		remotes     []remote
		expect      string
	}{
		{
			name:        "single origin",
			expectError: false,
			remotes: []remote{
				{name: "origin", url: "https://github.com/okteto/go-getting-started"},
			},
			expect: "https://github.com/okteto/go-getting-started",
		},
		{
			name:        "single remote",
			expectError: false,
			remotes: []remote{
				{name: "mine", url: "https://github.com/okteto/go-getting-started"},
			},
			expect: "https://github.com/okteto/go-getting-started",
		},
		{
			name:        "multiple remotes",
			expectError: false,
			remotes: []remote{
				{name: "fork", url: "https://github.com/oktetotest/go-getting-started"},
				{name: "origin", url: "https://github.com/cindy/go-getting-started"},
				{name: "upstream", url: "https://github.com/okteto/go-getting-started"},
			},
			expect: "https://github.com/cindy/go-getting-started",
		},
		{
			name:        "no remotes",
			expectError: true,
			remotes:     nil,
			expect:      "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			if _, err := model.GetRepositoryURL(dir); err == nil {

				t.Fatal("expected error when there's no github repo")
			}

			r, err := git.PlainInit(dir, true)
			if err != nil {
				t.Fatal(err)
			}

			for _, rm := range tt.remotes {
				if _, err := r.CreateRemote(&config.RemoteConfig{Name: rm.name, URLs: []string{rm.url}}); err != nil {
					t.Fatal(err)
				}
			}

			url, err := model.GetRepositoryURL(dir)

			if tt.expectError {
				if err == nil {
					t.Error("expected error when calling getRepositoryURL")
				}

				return
			}

			if err != nil {
				t.Fatal(err)
			}

			if url != tt.expect {
				t.Errorf("expected '%s', got '%s", tt.expect, url)
			}
		})
	}
}

func TestCheckAllResourcesRunning(t *testing.T) {

	var tests = []struct {
		name           string
		resourceStatus map[string]string
		expectError    bool
		expectResult   bool
	}{
		{
			name: "all-running",
			resourceStatus: map[string]string{
				"1": okteto.RunningStatus,
				"2": okteto.CompletedStatus,
				"3": okteto.NotRunningStatus,
			},
			expectError:  false,
			expectResult: true,
		},
		{
			name: "pulling",
			resourceStatus: map[string]string{
				"1": okteto.RunningStatus,
				"2": okteto.CompletedStatus,
				"3": okteto.NotRunningStatus,
				"4": okteto.PullingStatus,
			},
			expectError:  false,
			expectResult: false,
		},
		{
			name: "progressing",
			resourceStatus: map[string]string{
				"1": okteto.RunningStatus,
				"2": okteto.CompletedStatus,
				"3": okteto.NotRunningStatus,
				"4": okteto.ProgressingStatus,
			},
			expectError:  false,
			expectResult: false,
		},
		{
			name: "booting",
			resourceStatus: map[string]string{
				"1": okteto.RunningStatus,
				"2": okteto.CompletedStatus,
				"3": okteto.NotRunningStatus,
				"4": okteto.BootingStatus,
			},
			expectError:  false,
			expectResult: false,
		},
		{
			name: "error",
			resourceStatus: map[string]string{
				"1": okteto.RunningStatus,
				"2": okteto.CompletedStatus,
				"3": okteto.NotRunningStatus,
				"4": okteto.PullingStatus,
				"5": okteto.ProgressingStatus,
				"6": okteto.BootingStatus,
				"7": okteto.ErrorStatus,
			},
			expectError:  true,
			expectResult: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CheckAllResourcesRunning(tt.name, tt.resourceStatus)
			if tt.expectError && err == nil || !tt.expectError && err != nil {
				t.Errorf("expected error '%t', got '%v", tt.expectError, err)
			}
			if tt.expectResult != result {
				t.Errorf("expected result '%t', got '%t", tt.expectResult, result)
			}
		})
	}
}

func TestDeployPipelineSuccesful(t *testing.T) {
	ctx := context.Background()
	okteto.CurrentStore = &okteto.OktetoContextStore{
		CurrentContext: "test",
		Contexts: map[string]*okteto.OktetoContext{
			"test": {},
		},
	}
	response := &client.FakePipelineResponses{
		DeployResponse: &types.GitDeployResponse{
			Action: &types.Action{
				ID:   "test",
				Name: "test",
			},
		},
	}
	pc := &Command{
		okClient: &client.FakeOktetoClient{
			PipelineClient: client.NewFakePipelineClient(response),
		},
	}
	opts := &DeployOptions{
		Repository: "test",
		Name:       "test",
	}
	err := pc.ExecuteDeployPipeline(ctx, opts)
	assert.NoError(t, err)
}

func TestDeployPipelineSuccesfulWithWait(t *testing.T) {
	ctx := context.Background()
	okteto.CurrentStore = &okteto.OktetoContextStore{
		CurrentContext: "test",
		Contexts: map[string]*okteto.OktetoContext{
			"test": {},
		},
	}
	response := &client.FakePipelineResponses{
		DeployResponse: &types.GitDeployResponse{
			Action: &types.Action{
				ID:   "test",
				Name: "test",
			},
		},
		ResourcesMap: map[string]string{
			"svc":  okteto.CompletedStatus,
			"svc2": okteto.RunningStatus,
		},
	}

	pc := &Command{
		okClient: &client.FakeOktetoClient{
			PipelineClient: client.NewFakePipelineClient(response),
		},
	}
	opts := &DeployOptions{
		Repository: "test",
		Name:       "test",
		Wait:       true,
		Timeout:    2 * time.Second,
	}
	err := pc.ExecuteDeployPipeline(ctx, opts)
	assert.NoError(t, err)
}

func TestDeployWithError(t *testing.T) {
	ctx := context.Background()
	okteto.CurrentStore = &okteto.OktetoContextStore{
		CurrentContext: "test",
		Contexts: map[string]*okteto.OktetoContext{
			"test": {},
		},
	}
	deployErr := fmt.Errorf("error deploying test")
	response := &client.FakePipelineResponses{
		DeployErr: deployErr,
	}
	pc := &Command{
		okClient: &client.FakeOktetoClient{
			PipelineClient: client.NewFakePipelineClient(response),
		},
	}
	opts := &DeployOptions{
		Repository: "test",
		Name:       "test",
	}
	err := pc.ExecuteDeployPipeline(ctx, opts)
	assert.ErrorIs(t, err, deployErr)
}

func TestDeployPipelineSuccesfulWithWaitStreamError(t *testing.T) {
	ctx := context.Background()
	okteto.CurrentStore = &okteto.OktetoContextStore{
		CurrentContext: "test",
		Contexts: map[string]*okteto.OktetoContext{
			"test": {},
		},
	}
	response := &client.FakePipelineResponses{
		DeployResponse: &types.GitDeployResponse{
			Action: &types.Action{
				ID:   "test",
				Name: "test",
			},
		},
		ResourcesMap: map[string]string{
			"svc":  okteto.CompletedStatus,
			"svc2": okteto.RunningStatus,
		},
		StreamErr: errors.New("error"),
	}

	pc := &Command{
		okClient: &client.FakeOktetoClient{
			PipelineClient: client.NewFakePipelineClient(response),
		},
	}
	opts := &DeployOptions{
		Repository: "test",
		Name:       "test",
		Wait:       true,
		Timeout:    2 * time.Second,
	}
	err := pc.ExecuteDeployPipeline(ctx, opts)
	assert.NoError(t, err)
}
