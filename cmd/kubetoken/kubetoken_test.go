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

package kubetoken

import (
	"context"
	"testing"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/internal/test/client"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type fakeK8sClientProvider struct {
	client kubernetes.Interface
	err    error
}

func (f fakeK8sClientProvider) Provide(clientApiConfig *clientcmdapi.Config) (kubernetes.Interface, *rest.Config, error) {
	return f.client, nil, f.err
}

type fakeOktetoClientProvider struct {
	client types.OktetoInterface
	err    error
}

func (f fakeOktetoClientProvider) Provide(...okteto.Option) (types.OktetoInterface, error) {
	return f.client, f.err
}

type fakeCtxCmdRunner struct {
	err error
}

func (f fakeCtxCmdRunner) Run(ctx context.Context, ctxOptions *contextCMD.ContextOptions) error {
	return f.err
}

func TestKubetoken(t *testing.T) {
	ctx := context.Background()
	type input struct {
		flags                    KubetokenFlags
		contextStore             *okteto.OktetoContextStore
		fakeOktetoClientProvider fakeOktetoClientProvider
		fakeCtxCmdRunner         fakeCtxCmdRunner
	}

	fakeCtxStore := &okteto.OktetoContextStore{
		CurrentContext: "https://okteto.dev",
		Contexts: map[string]*okteto.OktetoContext{
			"https://okteto.dev": {
				IsOkteto: true,
			},
		},
	}

	tt := []struct {
		name     string
		input    input
		expected error
	}{
		{
			name: "error on validation",
			input: input{
				flags: KubetokenFlags{
					Context: "",
				},
			},
			expected: errEmptyContext,
		},
		{
			name: "error on context command Run",
			input: input{
				flags: KubetokenFlags{
					Context: "https://okteto.dev",
				},
				contextStore: fakeCtxStore,
				fakeOktetoClientProvider: fakeOktetoClientProvider{
					client: &client.FakeOktetoClient{
						KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{
							Token: types.KubeTokenResponse{},
						}),
					},
				},
				fakeCtxCmdRunner: fakeCtxCmdRunner{
					err: assert.AnError,
				},
			},
			expected: assert.AnError,
		},
		{
			name: "error getting kubetoken",
			input: input{
				flags: KubetokenFlags{
					Context: "https://okteto.dev",
				},
				contextStore: fakeCtxStore,
				fakeOktetoClientProvider: fakeOktetoClientProvider{
					client: &client.FakeOktetoClient{
						KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{
							Err: assert.AnError,
						}),
					},
				},
				fakeCtxCmdRunner: fakeCtxCmdRunner{
					err: assert.AnError,
				},
			},
			expected: assert.AnError,
		},
		{
			name: "successful",
			input: input{
				flags: KubetokenFlags{
					Context:   "https://okteto.dev",
					Namespace: "namespace",
				},
				contextStore: fakeCtxStore,
				fakeOktetoClientProvider: fakeOktetoClientProvider{
					client: &client.FakeOktetoClient{
						KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{
							Token: types.KubeTokenResponse{},
						}),
					},
				},
				fakeCtxCmdRunner: fakeCtxCmdRunner{},
			},
			expected: nil,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			cmd := NewKubetokenCmd()
			cmd.ctxStore = tc.input.contextStore
			cmd.oktetoClientProvider = tc.input.fakeOktetoClientProvider
			cmd.oktetoCtxCmdRunner = tc.input.fakeCtxCmdRunner
			cmd.ctxStore = tc.input.contextStore
			cmd.initCtxFunc = func(string, string) *contextCMD.ContextOptions {
				return &contextCMD.ContextOptions{
					Context:   tc.input.flags.Context,
					Namespace: tc.input.flags.Namespace,
				}
			}

			err := cmd.Run(ctx, tc.input.flags)
			assert.ErrorIs(t, err, tc.expected)
		})
	}
}
