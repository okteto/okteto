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

package context

import (
	"os"
	"testing"

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/internal/test/client"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/tools/clientcmd/api"
)

func Test_ExecuteUpdateKubeconfig_DisabledKubetoken(t *testing.T) {

	var tests = []struct {
		name             string
		kubeconfigCtx    test.KubeconfigFields
		context          *okteto.OktetoContextStore
		okClientProvider types.OktetoClientProvider
	}{
		{
			name: "change current ctx",
			kubeconfigCtx: test.KubeconfigFields{
				Name:           []string{"test", "to-change"},
				Namespace:      []string{"test", "test"},
				CurrentContext: "test",
			},
			context: &okteto.OktetoContextStore{
				CurrentContext: "to-change",
				Contexts: map[string]*okteto.OktetoContext{
					"to-change": {
						Namespace: "test",
						Cfg: &api.Config{
							CurrentContext: "to-change",
							Contexts: map[string]*api.Context{
								"to-change": {
									Namespace: "test",
								},
							},
						},
					},
				},
			},
			okClientProvider: client.NewFakeOktetoClientProvider(
				&client.FakeOktetoClient{
					KubetokenClient: client.NewFakeKubetokenClient(
						client.FakeKubetokenResponse{
							Err: assert.AnError,
						},
					),
				},
			),
		},
		{
			name: "change current namespace",
			kubeconfigCtx: test.KubeconfigFields{
				Name:           []string{"to-change"},
				Namespace:      []string{"test"},
				CurrentContext: "to-change",
			},
			context: &okteto.OktetoContextStore{
				CurrentContext: "to-change",
				Contexts: map[string]*okteto.OktetoContext{
					"to-change": {
						Namespace: "to-change",
						Cfg: &api.Config{
							CurrentContext: "to-change",
							Contexts: map[string]*api.Context{
								"to-change": {
									Namespace: "to-change",
								},
							},
						},
					},
				},
			},
			okClientProvider: client.NewFakeOktetoClientProvider(
				&client.FakeOktetoClient{
					KubetokenClient: client.NewFakeKubetokenClient(
						client.FakeKubetokenResponse{
							Err: assert.AnError,
						},
					),
				},
			),
		},
		{
			name:          "create if it doesn't exist",
			kubeconfigCtx: test.KubeconfigFields{},
			context: &okteto.OktetoContextStore{
				CurrentContext: "to-change",
				Contexts: map[string]*okteto.OktetoContext{
					"to-change": {
						Namespace: "to-change",
						Cfg: &api.Config{
							CurrentContext: "to-change",
							Contexts: map[string]*api.Context{
								"to-change": {
									Namespace: "to-change",
								},
							},
						},
					},
				},
			},
			okClientProvider: client.NewFakeOktetoClientProvider(
				&client.FakeOktetoClient{
					KubetokenClient: client.NewFakeKubetokenClient(
						client.FakeKubetokenResponse{
							Err: assert.AnError,
						},
					),
				},
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			okteto.CurrentStore = tt.context
			file, err := test.CreateKubeconfig(tt.kubeconfigCtx)
			if err != nil {
				assert.NoError(t, err, "error creating temporary kubeconfig")
			}
			defer os.Remove(file)

			okContext := okteto.Context()
			kubeconfigPaths := []string{file}

			err = ExecuteUpdateKubeconfig(okContext, kubeconfigPaths, tt.okClientProvider)
			assert.NoError(t, err, "error writing kubeconfig")

			cfg := kubeconfig.Get(kubeconfigPaths)
			assert.NotNil(t, cfg, "kubeconfig is nil")
			assert.Equal(t, tt.context.CurrentContext, cfg.CurrentContext, "current context has changed")
			assert.Equal(t, tt.context.Contexts[tt.context.CurrentContext].Namespace, cfg.Contexts[tt.context.CurrentContext].Namespace, "namespace has changed")

		})
	}
}

func Test_ExecuteUpdateKubeconfig_EnabledKubetoken(t *testing.T) {
	var tests = []struct {
		name          string
		kubeconfigCtx test.KubeconfigFields
		context       *okteto.OktetoContextStore
	}{
		{
			name: "change current ctx",
			kubeconfigCtx: test.KubeconfigFields{
				Name:           []string{"test", "to-change"},
				Namespace:      []string{"test", "test"},
				CurrentContext: "test",
			},
			context: &okteto.OktetoContextStore{
				CurrentContext: "to-change",
				Contexts: map[string]*okteto.OktetoContext{
					"to-change": {
						Namespace: "test",
						Cfg: &api.Config{
							CurrentContext: "to-change",
							Contexts: map[string]*api.Context{
								"to-change": {
									Namespace: "test",
								},
							},
						},
						IsOkteto: true,
					},
				},
			},
		},
		{
			name: "change current namespace",
			kubeconfigCtx: test.KubeconfigFields{
				Name:           []string{"to-change"},
				Namespace:      []string{"test"},
				CurrentContext: "to-change",
			},
			context: &okteto.OktetoContextStore{
				CurrentContext: "to-change",
				Contexts: map[string]*okteto.OktetoContext{
					"to-change": {
						Namespace: "to-change",
						Cfg: &api.Config{
							CurrentContext: "to-change",
							Contexts: map[string]*api.Context{
								"to-change": {
									Namespace: "to-change",
								},
							},
						},
						IsOkteto: true,
					},
				},
			},
		},
		{
			name:          "create if it doesn't exist",
			kubeconfigCtx: test.KubeconfigFields{},
			context: &okteto.OktetoContextStore{
				CurrentContext: "to-change",
				Contexts: map[string]*okteto.OktetoContext{
					"to-change": {
						Namespace: "to-change",
						Cfg: &api.Config{
							CurrentContext: "to-change",
							Contexts: map[string]*api.Context{
								"to-change": {
									Namespace: "to-change",
								},
							},
						},
						IsOkteto: true,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(oktetoUseStaticKubetokenEnvVar, "false")

			okClientProvider := client.NewFakeOktetoClientProvider(
				&client.FakeOktetoClient{
					KubetokenClient: client.NewFakeKubetokenClient(
						client.FakeKubetokenResponse{},
					),
				},
			)

			okteto.CurrentStore = tt.context
			file, err := test.CreateKubeconfig(tt.kubeconfigCtx)
			if err != nil {
				assert.NoError(t, err, "error creating temporary kubeconfig")
			}
			defer os.Remove(file)

			okContext := okteto.Context()
			kubeconfigPaths := []string{file}

			err = ExecuteUpdateKubeconfig(okContext, kubeconfigPaths, okClientProvider)
			assert.NoError(t, err, "error writing kubeconfig")

			cfg := kubeconfig.Get(kubeconfigPaths)
			assert.NotNil(t, cfg, "kubeconfig is nil")
			assert.Equal(t, tt.context.CurrentContext, cfg.CurrentContext, "current context has changed")
			assert.Equal(t, tt.context.Contexts[tt.context.CurrentContext].Namespace, cfg.Contexts[tt.context.CurrentContext].Namespace, "namespace has changed")
			assert.NotNil(t, cfg.AuthInfos)
			assert.Len(t, cfg.AuthInfos, 1)
			assert.NotNil(t, cfg.AuthInfos[""].Exec)
			assert.Empty(t, cfg.AuthInfos[""].Token)
		})
	}
}

func Test_RemoveExecFromCfg(t *testing.T) {
	var tests = []struct {
		name     string
		input    *okteto.OktetoContext
		expected *okteto.OktetoContext
	}{
		{
			name:     "nil context",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty UserID",
			input:    &okteto.OktetoContext{},
			expected: &okteto.OktetoContext{},
		},
		{
			name:     "nil config",
			input:    &okteto.OktetoContext{UserID: "test-user"},
			expected: &okteto.OktetoContext{UserID: "test-user"},
		},
		{
			name:     "nil AuthInfos",
			input:    &okteto.OktetoContext{UserID: "test-user", Cfg: &api.Config{}},
			expected: &okteto.OktetoContext{UserID: "test-user", Cfg: &api.Config{}},
		},
		{
			name:     "missing user in AuthInfos",
			input:    &okteto.OktetoContext{UserID: "test-user", Cfg: &api.Config{AuthInfos: make(map[string]*api.AuthInfo)}},
			expected: &okteto.OktetoContext{UserID: "test-user", Cfg: &api.Config{AuthInfos: make(map[string]*api.AuthInfo)}},
		},
		{
			name: "Exec removed successfully",
			input: &okteto.OktetoContext{
				UserID: "test-user",
				Cfg: &api.Config{AuthInfos: map[string]*api.AuthInfo{
					"test-user": {Token: "test-token", Exec: &api.ExecConfig{Command: "test-cmd"}},
				}},
			},
			expected: &okteto.OktetoContext{
				UserID: "test-user",
				Cfg: &api.Config{AuthInfos: map[string]*api.AuthInfo{
					"test-user": {Token: "test-token", Exec: nil},
				}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			removeExecFromCfg(tt.input)

			assert.Equal(t, tt.expected, tt.input)
		})
	}
}

func Test_ExecuteUpdateKubeconfig_With_OktetoUseStaticKubetokenEnvVar(t *testing.T) {
	t.Setenv(oktetoUseStaticKubetokenEnvVar, "true")

	okteto.CurrentStore = &okteto.OktetoContextStore{
		CurrentContext: "ctx-test",
		Contexts: map[string]*okteto.OktetoContext{
			"ctx-test": {
				UserID:    "test-user",
				Namespace: "ns-text",
				Cfg: &api.Config{
					AuthInfos: map[string]*api.AuthInfo{
						"test-user": {Token: "test-token", Exec: &api.ExecConfig{Command: "test-cmd"}},
					},
				},
				IsOkteto: true,
			},
		},
	}

	file, err := test.CreateKubeconfig(test.KubeconfigFields{
		Name:           []string{"name-test"},
		Namespace:      []string{"ns-test"},
		CurrentContext: "ctx-test",
	})

	if err != nil {
		assert.NoError(t, err, "error creating temporary kubeconfig")
	}
	defer os.Remove(file)

	okContext := okteto.Context()
	kubeconfigPaths := []string{file}
	okClientProvider := client.NewFakeOktetoClientProvider(
		&client.FakeOktetoClient{
			KubetokenClient: client.NewFakeKubetokenClient(
				client.FakeKubetokenResponse{},
			),
		},
	)

	err = ExecuteUpdateKubeconfig(okContext, kubeconfigPaths, okClientProvider)
	assert.NoError(t, err, "error writing kubeconfig")

	cfg := kubeconfig.Get(kubeconfigPaths)
	assert.Nil(t, cfg.AuthInfos["test-user"].Exec)
	assert.Equal(t, cfg.AuthInfos["test-user"].Token, "test-token")
}

func Test_ExecuteUpdateKubeconfig_ForNonOktetoContext(t *testing.T) {
	okteto.CurrentStore = &okteto.OktetoContextStore{
		CurrentContext: "ctx-test",
		Contexts: map[string]*okteto.OktetoContext{
			"ctx-test": {
				UserID:    "test-user",
				Namespace: "ns-text",
				Cfg: &api.Config{
					AuthInfos: map[string]*api.AuthInfo{
						"test-user": {Exec: &api.ExecConfig{Command: "test-cmd"}},
					},
				},
				IsOkteto: false,
			},
		},
	}

	file, err := test.CreateKubeconfig(test.KubeconfigFields{
		Name:           []string{"name-test"},
		Namespace:      []string{"ns-test"},
		CurrentContext: "ctx-test",
	})

	if err != nil {
		assert.NoError(t, err, "error creating temporary kubeconfig")
	}
	defer os.Remove(file)

	okContext := okteto.Context()
	kubeconfigPaths := []string{file}

	err = ExecuteUpdateKubeconfig(okContext, kubeconfigPaths, nil)
	assert.NoError(t, err, "error writing kubeconfig")

	cfg := kubeconfig.Get(kubeconfigPaths)
	assert.NotNil(t, cfg.AuthInfos["test-user"].Exec)
}
