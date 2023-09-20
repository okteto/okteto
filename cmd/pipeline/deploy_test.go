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
	"github.com/okteto/okteto/pkg/model"
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
		k8sClientProvider: test.NewFakeK8sProvider(),
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
		Repository: "test",
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
		k8sClientProvider: test.NewFakeK8sProvider(),
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
		Repository: "test",
		Name:       "test",
		Namespace:  "test",
		Wait:       true,
		Timeout:    2 * time.Second,
	}
	err := pc.ExecuteDeployPipeline(ctx, opts)
	assert.NoError(t, err)
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
		name            string
		cmap            *v1.ConfigMap
		envSetter       fakeEnvSetter
		expectedErr     bool
		expectedEnvsSet map[string]string
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
				"OKTETO_DEPENDENCY_TEST-CONFIGMAP_VARIABLE_TESTSETENVSFROMDEPEN_ONE": "an env value",
				"OKTETO_DEPENDENCY_TEST-CONFIGMAP_VARIABLE_TESTSETENVSFROMDEPEN_TWO": "another env value",
			},
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
		name   string
		flags  deployFlags
		expect *DeployOptions
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
	override := DeployOptions{
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
		name        string
		inputLabels map[string]string
		inputData   map[string]string
		expected    DeployOptions
	}{
		{
			name:     "empty input",
			expected: DeployOptions{},
		},
		{
			name: "complete input",
			inputData: map[string]string{
				"name":       "test-name",
				"namespace":  "test-namespace",
				"filename":   "test-filename",
				"file":       "test-file",
				"repository": "test-repository",
				"branch":     "test-branch",
				"variables":  "",
				"other":      "not-exist",
			},
			inputLabels: map[string]string{
				"label.okteto.com/testing": "true",
				"ignored.label":            "true",
			},
			expected: DeployOptions{
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
			got := cfgToDeployOptions(tt.inputLabels, tt.inputData)
			require.Equal(t, tt.expected, got)
		})
	}
}

func Test_parseVariablesListFromCfgVariablesString(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedErr bool
		expected    []string
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
