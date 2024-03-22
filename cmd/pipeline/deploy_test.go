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
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/internal/test/client"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/model/utils"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiv1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_getRepositoryURL(t *testing.T) {

	type remote struct {
		name string
		url  string
	}
	var tests = []struct {
		name        string
		expect      string
		remotes     []remote
		expectError bool
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

			if _, err := utils.GetRepositoryURL(dir); err == nil {

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

			url, err := utils.GetRepositoryURL(dir)

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
	okteto.CurrentStore = &okteto.ContextStore{
		CurrentContext: "test",
		Contexts: map[string]*okteto.Context{
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
		k8sClientProvider: test.NewFakeK8sProvider(),
	}
	opts := &DeployOptions{
		Repository: "http://stest",
		Name:       "test",
	}
	err := pc.ExecuteDeployPipeline(ctx, opts)
	assert.NoError(t, err)
}

func TestDeployPipelineSuccesfulWithWait(t *testing.T) {
	ctx := context.Background()
	okteto.CurrentStore = &okteto.ContextStore{
		CurrentContext: "test",
		Contexts: map[string]*okteto.Context{
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

	cmap := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pipeline.TranslatePipelineName("test"),
			Namespace: "test",
			Labels:    map[string]string{},
		},
		Data: nil,
	}

	pc := &Command{
		okClient: &client.FakeOktetoClient{
			PipelineClient: client.NewFakePipelineClient(response),
			StreamClient:   client.NewFakeStreamClient(&client.FakeStreamResponse{}),
		},
		k8sClientProvider: test.NewFakeK8sProvider(cmap),
	}
	opts := &DeployOptions{
		Repository: "https://test",
		Name:       "test",
		Namespace:  "test",
		Wait:       true,
		Timeout:    2 * time.Second,
	}
	err := pc.ExecuteDeployPipeline(ctx, opts)
	assert.NoError(t, err)
}

func TestDeployWithError(t *testing.T) {
	ctx := context.Background()
	okteto.CurrentStore = &okteto.ContextStore{
		CurrentContext: "test",
		Contexts: map[string]*okteto.Context{
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
		k8sClientProvider: test.NewFakeK8sProvider(),
	}
	opts := &DeployOptions{
		Repository: "https://test",
		Name:       "test",
	}
	err := pc.ExecuteDeployPipeline(ctx, opts)
	assert.ErrorIs(t, err, deployErr)
}

func TestDeployPipelineSuccesfulWithWaitStreamError(t *testing.T) {
	ctx := context.Background()
	okteto.CurrentStore = &okteto.ContextStore{
		CurrentContext: "test",
		Contexts: map[string]*okteto.Context{
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

	cmap := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pipeline.TranslatePipelineName("test"),
			Namespace: "test",
			Labels:    map[string]string{},
		},
		Data: nil,
	}

	pc := &Command{
		okClient: &client.FakeOktetoClient{
			PipelineClient: client.NewFakePipelineClient(response),
			StreamClient:   client.NewFakeStreamClient(&client.FakeStreamResponse{StreamErr: errors.New("error")}),
		},
		k8sClientProvider: test.NewFakeK8sProvider(cmap),
	}
	opts := &DeployOptions{
		Repository: "https://test",
		Name:       "test",
		Namespace:  "test",
		Wait:       true,
		Timeout:    2 * time.Second,
	}
	err := pc.ExecuteDeployPipeline(ctx, opts)
	assert.NoError(t, err)
}

func Test_DeployPipelineWithReuseParamsNotFoundError(t *testing.T) {
	ctx := context.Background()
	okteto.CurrentStore = &okteto.ContextStore{
		CurrentContext: "test",
		Contexts: map[string]*okteto.Context{
			"test": {},
		},
	}
	pc := &Command{
		k8sClientProvider: test.NewFakeK8sProvider(),
	}
	opts := &DeployOptions{
		Repository:  "https://test",
		Name:        "test",
		ReuseParams: true,
	}
	err := pc.ExecuteDeployPipeline(ctx, opts)
	assert.ErrorIs(t, err, errUnableToReuseParams)
}

func Test_DeployPipelineWithReuseParamsSuccess(t *testing.T) {
	ctx := context.Background()
	okteto.CurrentStore = &okteto.ContextStore{
		CurrentContext: "test",
		Contexts: map[string]*okteto.Context{
			"test": {},
		},
	}

	fakePipelineDeployResponses := &client.FakePipelineResponses{}
	fakePipelineClient := client.NewFakePipelineClient(
		fakePipelineDeployResponses,
	)
	pc := &Command{
		okClient: &client.FakeOktetoClient{
			PipelineClient: fakePipelineClient,
		},
		k8sClientProvider: test.NewFakeK8sProvider(
			&v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "okteto-git-test",
					Namespace: "test",
					Labels: map[string]string{
						"label.okteto.com/labeltest": "true",
					},
				},
				Data: map[string]string{
					"branch":   "testing",
					"filename": "file",
				},
			},
		),
	}
	opts := &DeployOptions{
		Repository:  "https://test",
		Name:        "test",
		Namespace:   "test",
		ReuseParams: true,
	}
	err := pc.ExecuteDeployPipeline(ctx, opts)
	assert.NoError(t, err)

	assert.Equal(t, types.PipelineDeployOptions{
		Branch:    "testing",
		Filename:  "file",
		Labels:    []string{"labeltest"},
		Namespace: "test",
	}, fakePipelineDeployResponses.DeployOpts)
}

func Test_DeployPipelineWithSkipIfExist(t *testing.T) {
	okteto.CurrentStore = &okteto.ContextStore{
		CurrentContext: "test",
		Contexts: map[string]*okteto.Context{
			"test": {},
		},
	}

	fakePipelineClientResponses := &client.FakePipelineResponses{}

	tests := []struct {
		cmd  *Command
		opts *DeployOptions
		name string
	}{
		{
			name: "skip because deployed status",
			cmd: &Command{
				okClient: &client.FakeOktetoClient{
					PipelineClient: client.NewFakePipelineClient(
						fakePipelineClientResponses,
					),
				},
				k8sClientProvider: test.NewFakeK8sProvider(
					&v1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "okteto-git-test",
							Namespace: "test",
							Labels: map[string]string{
								"label.okteto.com/labeltest": "true",
							},
						},
						Data: map[string]string{
							"status":   pipeline.DeployedStatus,
							"branch":   "testing",
							"filename": "file",
						},
					},
				),
			},
			opts: &DeployOptions{
				Repository:   "https://test",
				Name:         "test",
				Namespace:    "test",
				SkipIfExists: true,
			},
		},
		{
			name: "skip because deployed status",
			cmd: &Command{
				okClient: &client.FakeOktetoClient{
					PipelineClient: client.NewFakePipelineClient(
						fakePipelineClientResponses,
					),
				},
				k8sClientProvider: test.NewFakeK8sProvider(
					&v1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "okteto-git-test",
							Namespace: "test",
							Labels: map[string]string{
								"label.okteto.com/labeltest": "true",
							},
						},
						Data: map[string]string{
							"status":   pipeline.ProgressingStatus,
							"branch":   "testing",
							"filename": "file",
						},
					},
				),
			},
			opts: &DeployOptions{
				Repository:   "https://test",
				Name:         "test",
				Namespace:    "test",
				SkipIfExists: true,
			},
		},
	}

	for _, tt := range tests {
		ctx := context.Background()

		t.Run(tt.name, func(t *testing.T) {
			err := tt.cmd.ExecuteDeployPipeline(ctx, tt.opts)
			require.NoError(t, err)
			require.Zero(t, fakePipelineClientResponses.CallCount)
		})
	}
}

func Test_DeployPipelineWithSkipIfExistAndWait(t *testing.T) {
	okteto.CurrentStore = &okteto.ContextStore{
		CurrentContext: "test",
		Contexts: map[string]*okteto.Context{
			"test": {},
		},
	}

	fakePipelineClientResponses := &client.FakePipelineResponses{}

	tests := []struct {
		cmd  *Command
		opts *DeployOptions
		name string
	}{
		{
			name: "wait and canStreamPrevLogs",
			cmd: &Command{
				okClient: &client.FakeOktetoClient{
					PipelineClient: client.NewFakePipelineClient(
						fakePipelineClientResponses,
					),
					StreamClient: client.NewFakeStreamClient(
						&client.FakeStreamResponse{},
					),
				},
				k8sClientProvider: test.NewFakeK8sProvider(
					&v1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "okteto-git-test",
							Namespace: "test",
							Labels: map[string]string{
								"label.okteto.com/labeltest": "true",
							},
						},
						Data: map[string]string{
							"branch":     "testing",
							"filename":   "file",
							"actionLock": "lock",
							"actionName": "not-cli",
						},
					},
				),
			},
			opts: &DeployOptions{
				Repository:   "https://test",
				Name:         "test",
				Namespace:    "test",
				SkipIfExists: true,
				Wait:         true,
				Timeout:      1 * time.Minute,
			},
		},
		{
			name: "wait and canStreamPrevLogs with err in streaming ignore err",
			cmd: &Command{
				okClient: &client.FakeOktetoClient{
					PipelineClient: client.NewFakePipelineClient(
						fakePipelineClientResponses,
					),
					StreamClient: client.NewFakeStreamClient(
						&client.FakeStreamResponse{
							StreamErr: assert.AnError,
						},
					),
				},
				k8sClientProvider: test.NewFakeK8sProvider(
					&v1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "okteto-git-test",
							Namespace: "test",
							Labels: map[string]string{
								"label.okteto.com/labeltest": "true",
							},
						},
						Data: map[string]string{
							"branch":     "testing",
							"filename":   "file",
							"actionLock": "lock",
							"actionName": "not-cli",
						},
					},
				),
			},
			opts: &DeployOptions{
				Repository:   "https://test",
				Name:         "test",
				Namespace:    "test",
				SkipIfExists: true,
				Wait:         true,
				Timeout:      1 * time.Minute,
			},
		},
		{
			name: "wait and not canStreamPrevLogs",
			cmd: &Command{
				okClient: &client.FakeOktetoClient{
					PipelineClient: client.NewFakePipelineClient(
						fakePipelineClientResponses,
					),
					StreamClient: client.NewFakeStreamClient(
						&client.FakeStreamResponse{},
					),
				},
				k8sClientProvider: test.NewFakeK8sProvider(
					&v1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "okteto-git-test",
							Namespace: "test",
							Labels: map[string]string{
								"label.okteto.com/labeltest": "true",
							},
						},
						Data: map[string]string{
							"status":     pipeline.DeployedStatus,
							"branch":     "testing",
							"filename":   "file",
							"actionLock": "",
							"actionName": "",
						},
					},
				),
			},
			opts: &DeployOptions{
				Repository:   "https://test",
				Name:         "test",
				Namespace:    "test",
				SkipIfExists: true,
				Wait:         true,
				Timeout:      1 * time.Minute,
			},
		},
	}

	for _, tt := range tests {
		ctx := context.Background()

		t.Run(tt.name, func(t *testing.T) {
			err := tt.cmd.ExecuteDeployPipeline(ctx, tt.opts)
			require.NoError(t, err)
			require.Zero(t, fakePipelineClientResponses.CallCount)
		})
	}
}

type fakeEnvSetter struct {
	envs map[string]string
	err  error
}

func (e *fakeEnvSetter) Set(name, value string) error {
	if e.err != nil {
		return e.err
	}
	if e.envs == nil {
		e.envs = make(map[string]string)
	}
	e.envs[name] = value
	return nil
}

func TestSetEnvsFromDependencyNoError(t *testing.T) {
	var tests = []struct {
		envSetter       fakeEnvSetter
		cmap            *v1.ConfigMap
		expectedEnvsSet map[string]string
		name            string
		expectedErr     bool
	}{
		{
			name:            "nil cmap",
			envSetter:       fakeEnvSetter{},
			expectedErr:     false,
			expectedEnvsSet: nil,
		},
		{
			name:      "configmap has no dependency envs",
			envSetter: fakeEnvSetter{},
			cmap: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-configmap",
				},
				Data: map[string]string{
					"variables": "eyJURVNUU0VURU5WU0ZST01ERVBFTl9PTkUiOiJhbiBlbnYgdmFsdWUiLCJURVNUU0VURU5WU0ZST01ERVBFTl9UV08iOiJhbm90aGVyIGVudiB2YWx1ZSJ9",
				},
			},
			expectedErr:     false,
			expectedEnvsSet: nil,
		},
		{
			name:      "configmap has dependency envs",
			envSetter: fakeEnvSetter{},
			cmap: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-configmap",
				},
				Data: map[string]string{
					constants.OktetoDependencyEnvsKey: "eyJURVNUU0VURU5WU0ZST01ERVBFTl9PTkUiOiJhbiBlbnYgdmFsdWUiLCJURVNUU0VURU5WU0ZST01ERVBFTl9UV08iOiJhbm90aGVyIGVudiB2YWx1ZSJ9",
				},
			},
			expectedErr: false,
			expectedEnvsSet: map[string]string{
				"OKTETO_DEPENDENCY_TEST_CONFIGMAP_VARIABLE_TESTSETENVSFROMDEPEN_ONE": "an env value",
				"OKTETO_DEPENDENCY_TEST_CONFIGMAP_VARIABLE_TESTSETENVSFROMDEPEN_TWO": "another env value",
			},
		},
		{
			name:      "okteto git configmap has dependency envs",
			envSetter: fakeEnvSetter{},
			cmap: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "okteto-git-test-configmap",
				},
				Data: map[string]string{
					constants.OktetoDependencyEnvsKey: "eyJURVNUU0VURU5WU0ZST01ERVBFTl9PTkUiOiJhbiBlbnYgdmFsdWUiLCJURVNUU0VURU5WU0ZST01ERVBFTl9UV08iOiJhbm90aGVyIGVudiB2YWx1ZSJ9",
				},
			},
			expectedErr: false,
			expectedEnvsSet: map[string]string{
				"OKTETO_DEPENDENCY_TEST_CONFIGMAP_VARIABLE_TESTSETENVSFROMDEPEN_ONE": "an env value",
				"OKTETO_DEPENDENCY_TEST_CONFIGMAP_VARIABLE_TESTSETENVSFROMDEPEN_TWO": "another env value",
			},
		},
		{
			name: "configmap has dependency envs - return err",
			envSetter: fakeEnvSetter{
				err: assert.AnError,
			},
			cmap: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-configmap",
				},
				Data: map[string]string{
					constants.OktetoDependencyEnvsKey: "eyJURVNUU0VURU5WU0ZST01ERVBFTl9PTkUiOiJhbiBlbnYgdmFsdWUiLCJURVNUU0VURU5WU0ZST01ERVBFTl9UV08iOiJhbm90aGVyIGVudiB2YWx1ZSJ9",
				},
			},
			expectedErr:     true,
			expectedEnvsSet: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := setEnvsFromDependency(tt.cmap, tt.envSetter.Set)
			require.Truef(t, tt.expectedErr == (err != nil), "unexpected error")
			require.Equal(t, tt.expectedEnvsSet, tt.envSetter.envs)
		})
	}
}

func TestFlagsToOptions(t *testing.T) {
	tt := []struct {
		expect *DeployOptions
		name   string
		flags  deployFlags
	}{
		{
			name:   "no flags",
			flags:  deployFlags{},
			expect: &DeployOptions{},
		},
		{
			name: "filename and file",
			flags: deployFlags{
				file:     "file",
				filename: "filename",
			},
			expect: &DeployOptions{
				File: "file",
			},
		},
		{
			name: "just filename",
			flags: deployFlags{
				filename: "filename",
			},
			expect: &DeployOptions{
				File: "filename",
			},
		},
		{
			name: "all flags ",
			flags: deployFlags{
				branch:      "branch",
				repository:  "repository",
				name:        "name",
				namespace:   "namespace",
				wait:        true,
				timeout:     2 * time.Second,
				labels:      []string{"label1", "label2"},
				variables:   []string{"var1=1", "var2=2"},
				reuseParams: true,
			},
			expect: &DeployOptions{
				Branch:      "branch",
				Repository:  "repository",
				Name:        "name",
				Namespace:   "namespace",
				Wait:        true,
				Timeout:     2 * time.Second,
				Labels:      []string{"label1", "label2"},
				Variables:   []string{"var1=1", "var2=2"},
				ReuseParams: true,
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			opts := tc.flags.toOptions()
			assert.Equal(t, tc.expect, opts)
		})
	}
}

func Test_applyOverrideToOptions(t *testing.T) {
	opts := &DeployOptions{
		Wait:    true,
		Timeout: 1 * time.Minute,
	}
	override := &DeployOptions{
		Name:       "test",
		Namespace:  "testing",
		File:       "filename",
		Repository: "repository",
		Branch:     "branch",
		Variables:  []string{"KEY=value"},
		Labels:     []string{"testlabel"},
	}

	applyOverrideToOptions(opts, override)

	require.Equal(t, &DeployOptions{
		Name:       "test",
		Namespace:  "testing",
		File:       "filename",
		Repository: "repository",
		Branch:     "branch",
		Variables:  []string{"KEY=value"},
		Labels:     []string{"testlabel"},
		Wait:       true,
		Timeout:    1 * time.Minute,
	}, opts)
}

func Test_cfgToDeployOptions(t *testing.T) {
	tests := []struct {
		input    *v1.ConfigMap
		expected *DeployOptions
		name     string
	}{
		{
			name:     "empty input",
			expected: nil,
		},
		{
			name: "empty namespace",
			input: &v1.ConfigMap{
				Data: map[string]string{
					"name":       "test-name",
					"filename":   "test-filename",
					"file":       "test-file",
					"repository": "test-repository",
					"branch":     "test-branch",
					"variables":  "",
				},
			},
			expected: &DeployOptions{
				Name:       "test-name",
				File:       "test-filename",
				Repository: "test-repository",
				Branch:     "test-branch",
			},
		},
		{
			name: "with labels and namespace",
			input: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
					Labels: map[string]string{
						"label.okteto.com/testing": "true",
						"ignored.label":            "true",
					},
				},
				Data: map[string]string{
					"name":       "test-name",
					"namespace":  "test-namespace",
					"filename":   "test-filename",
					"file":       "test-file",
					"repository": "test-repository",
					"branch":     "test-branch",
					"variables":  "",
				},
			},
			expected: &DeployOptions{
				Name:       "test-name",
				Namespace:  "test-namespace",
				File:       "test-filename",
				Repository: "test-repository",
				Branch:     "test-branch",
				Labels:     []string{"testing"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfgToDeployOptions(tt.input)
			require.Equal(t, tt.expected, got)
		})
	}
}

func Test_parseVariablesListFromCfgVariablesString(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    []string
		expectedErr bool
	}{
		{
			name:     "empty input",
			input:    "",
			expected: nil,
		},
		{
			name:        "invalid input",
			input:       "not-encoded-string",
			expectedErr: true,
			expected:    nil,
		},
		{
			name:     "valid input",
			input:    "W3sibmFtZSI6IktFWSIsInZhbHVlIjoidmFsdWUifV0=",
			expected: []string{"KEY=value"},
		},
	}

	for _, tt := range tests {
		got, err := parseVariablesListFromCfgVariablesString(tt.input)
		require.Truef(t, tt.expectedErr == (err != nil), fmt.Sprintf("got unexpected error %v", err))
		require.Equal(t, tt.expected, got)
	}
}

func Test_parseEnvironmentLabelFromLabelsMap(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		expected []string
	}{
		{
			name:  "empty input",
			input: nil,
		},
		{
			name: "with environment labels input",
			input: map[string]string{
				"label.okteto.com":         "true",
				"label.okteto.com/":        "true",
				"label.okteto.com/testing": "true",
				"dev.okteto.com":           "true",
			},
			expected: []string{"testing"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseEnvironmentLabelFromLabelsMap(tt.input)

			require.Equal(t, got, tt.expected)
		})
	}
}
