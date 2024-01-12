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

package pipeline

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/internal/test/client"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd/api"
)

type fakeOkCtxController struct {
	ns  string
	cfg *api.Config
}

func (f fakeOkCtxController) GetNamespace() string      { return f.ns }
func (f fakeOkCtxController) GetK8sConfig() *api.Config { return f.cfg }

func TestCheckAllResourcesRunning(t *testing.T) {
	var tests = []struct {
		resourceStatus map[string]string
		name           string
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
			d := &Deployer{
				ioCtrl: io.NewIOController(),
			}
			result, err := d.checkAllResourcesRunning(tt.name, tt.resourceStatus)
			if tt.expectError && err == nil || !tt.expectError && err != nil {
				t.Errorf("expected error '%t', got '%v", tt.expectError, err)
			}
			if tt.expectResult != result {
				t.Errorf("expected result '%t', got '%t", tt.expectResult, result)
			}
		})
	}
}

func TestWaitForResourcesToBeRunning(t *testing.T) {
	var tests = []struct {
		name         string
		timeout      time.Duration
		pipelineCtrl types.PipelineInterface
		err          error
	}{
		{
			name:    "timeout error",
			timeout: 1 * time.Nanosecond,
			err:     &ErrTimeout{"", 1 * time.Nanosecond},
		},
		{
			name:    "error getting resources",
			timeout: 5 * time.Second,
			pipelineCtrl: client.NewFakePipelineClient(&client.FakePipelineResponses{
				ResourceErr: assert.AnError,
			}),
			err: assert.AnError,
		},
		{
			name:    "error on one",
			timeout: 5 * time.Second,
			pipelineCtrl: client.NewFakePipelineClient(&client.FakePipelineResponses{
				ResourcesMap: map[string]string{
					"1": okteto.ErrorStatus,
				},
			}),
			err: fmt.Errorf("repository '' deployed with errors"),
		},
		{
			name:    "all running",
			timeout: 5 * time.Second,
			pipelineCtrl: client.NewFakePipelineClient(&client.FakePipelineResponses{
				ResourcesMap: map[string]string{
					"1": okteto.RunningStatus,
				},
			}),
			err: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Deployer{
				ioCtrl:       io.NewIOController(),
				pipelineCtrl: tt.pipelineCtrl,
			}
			err := d.waitForResourcesToBeRunning(context.Background(), "", "", tt.timeout)
			if err != nil {
				assert.ErrorContains(t, err, tt.err.Error())
			} else {
				assert.NoError(t, err)
			}

		})
	}
}

func TestWaitUntilRunning(t *testing.T) {
	var tests = []struct {
		name         string
		timeout      time.Duration
		pipelineCtrl types.PipelineInterface
		streamCtrl   types.StreamInterface
		err          error
	}{
		{
			name:    "without streaming",
			timeout: 5 * time.Second,
			pipelineCtrl: client.NewFakePipelineClient(&client.FakePipelineResponses{
				ResourcesMap: map[string]string{
					"1": okteto.RunningStatus,
				},
			}),
			streamCtrl: client.NewFakeStreamClient(&client.FakeStreamResponse{
				StreamErr: assert.AnError,
			}),
		},
		{
			name:    "waitErr",
			timeout: 5 * time.Second,
			pipelineCtrl: client.NewFakePipelineClient(&client.FakePipelineResponses{
				WaitErr: assert.AnError,
			}),
			streamCtrl: client.NewFakeStreamClient(&client.FakeStreamResponse{}),
			err:        assert.AnError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Deployer{
				ioCtrl:       io.NewIOController(),
				pipelineCtrl: tt.pipelineCtrl,
				streamCtrl:   tt.streamCtrl,
			}
			err := d.waitUntilRunning(context.Background(), "", "", &types.Action{}, tt.timeout)
			if err != nil {
				assert.ErrorContains(t, err, tt.err.Error())
			} else {
				assert.NoError(t, err)
			}

		})
	}
}

func TestDeploy(t *testing.T) {
	okCtx := fakeOkCtxController{
		ns: "test",
		cfg: &api.Config{
			CurrentContext: "test",
		},
	}

	cmapDeployed := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "okteto-git-test",
			Namespace: "test",
		},
		Data: map[string]string{
			"repository": "test",
			"branch":     "test",
			"commit":     "test",
			"status":     pipeline.DeployedStatus,
		},
	}
	cmapProgressing := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "okteto-git-test",
			Namespace: "test",
		},
		Data: map[string]string{
			"repository": "test",
			"branch":     "test",
			"commit":     "test",
			"status":     pipeline.ProgressingStatus,
		},
	}
	cmap := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "okteto-git-test",
			Namespace: "test",
		},
		Data: map[string]string{
			"repository": "test",
			"branch":     "test",
			"commit":     "test",
		},
	}
	var tests = []struct {
		name              string
		k8sClientProvider okteto.K8sClientProvider
		opts              *Options
		pipelineCtrl      types.PipelineInterface
		streamCtrl        types.StreamInterface
		err               error
	}{
		{
			name:              "error providing",
			k8sClientProvider: &test.FakeK8sProvider{ErrProvide: assert.AnError},
			err:               assert.AnError,
		},
		{
			name:              "error unable to reuse params",
			k8sClientProvider: test.NewFakeK8sProvider(),
			opts:              &Options{ReuseParams: true},
			err:               errUnableToReuseParams,
		},
		{
			name:              "alreayd deployed",
			k8sClientProvider: test.NewFakeK8sProvider(cmapDeployed),
			opts:              &Options{SkipIfExists: true, Namespace: "test", Name: "test"},
			pipelineCtrl:      client.NewFakePipelineClient(&client.FakePipelineResponses{}),
			err:               errUnableToReuseParams,
		},
		{
			name:              "alreayd progressing",
			k8sClientProvider: test.NewFakeK8sProvider(cmapProgressing),
			opts:              &Options{ReuseParams: true, SkipIfExists: true, Namespace: "test", Name: "test"},
			pipelineCtrl:      client.NewFakePipelineClient(&client.FakePipelineResponses{}),
			err:               errUnableToReuseParams,
		},
		{
			name:              "deploy correctly without wait",
			k8sClientProvider: test.NewFakeK8sProvider(cmap),
			opts:              &Options{Namespace: "test", Name: "test"},
			pipelineCtrl:      client.NewFakePipelineClient(&client.FakePipelineResponses{}),
			err:               errUnableToReuseParams,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Deployer{
				k8sClientProvider: tt.k8sClientProvider,
				ioCtrl:            io.NewIOController(),
				pipelineCtrl:      tt.pipelineCtrl,
				streamCtrl:        tt.streamCtrl,
				okCtxController:   okCtx,
				opts:              tt.opts,
			}
			err := d.Deploy(context.Background())
			if err != nil {
				assert.ErrorContains(t, err, tt.err.Error())
			} else {
				assert.NoError(t, err)
			}

		})
	}
}

func TestSetEnvsFromDependency(t *testing.T) {
	var tests = []struct {
		name string
		cmap *apiv1.ConfigMap
		err  bool
	}{
		{
			name: "nil cmap",
			err:  false,
		},
		{
			name: "cmap without depnedency",
			cmap: &apiv1.ConfigMap{
				Data: map[string]string{},
			},
			err: false,
		},
		{
			name: "error decoding",
			cmap: &apiv1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "okteto-git-test",
					Namespace: "test",
				},
				Data: map[string]string{
					constants.OktetoDependencyEnvsKey: `{"envs":[{"name":"t}est","value":"test"}]`,
				},
			},
			err: true,
		},
		{
			name: "ok",
			cmap: &apiv1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "okteto-git-test",
					Namespace: "test",
				},
				Data: map[string]string{
					constants.OktetoDependencyEnvsKey: `eyJuYW1lIjoidGVzdCIsInZhbHVlIjoidGVzdCJ9`,
				},
			},
			err: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := setEnvsFromDependency(tt.cmap, os.Setenv)
			if tt.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
