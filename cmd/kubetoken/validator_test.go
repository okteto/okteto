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
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type fakeK8sClientProvider struct {
	client kubernetes.Interface
	err    error
}

func (f fakeK8sClientProvider) Provide(_ *clientcmdapi.Config) (kubernetes.Interface, *rest.Config, error) {
	return f.client, nil, f.err
}

type fakeOktetoClientProvider struct {
	client types.OktetoInterface
	err    error
}

func (f fakeOktetoClientProvider) Provide(...okteto.Option) (types.OktetoInterface, error) {
	return f.client, f.err
}

func TestPreReqValidator(t *testing.T) {
	type input struct {
		ctxName              string
		ns                   string
		k8sClientProvider    okteto.K8sClientProvider
		oktetoClientProvider oktetoClientProvider
		ctx                  context.Context
		currentStore         *okteto.OktetoContextStore
	}

	ctx := context.Background()
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	tt := []struct {
		name     string
		input    input
		expected error
	}{
		{
			name: "fail on context validation",
			input: input{
				ctxName: "",
				ctx:     ctx,
				currentStore: &okteto.OktetoContextStore{
					Contexts:       map[string]*okteto.OktetoContext{},
					CurrentContext: "",
				},
			},
			expected: errEmptyContext,
		},
		{
			name: "fail on okteto client validation",
			input: input{
				ctxName: "https://okteto.com",
				ns:      "test",
				currentStore: &okteto.OktetoContextStore{
					Contexts: map[string]*okteto.OktetoContext{
						"https://okteto.com": {
							IsOkteto: true,
						},
						"k8s_cluster": {},
					},
					CurrentContext: "https://okteto.com",
				},
				k8sClientProvider: fakeK8sClientProvider{err: assert.AnError},
				oktetoClientProvider: fakeOktetoClientProvider{
					err: assert.AnError,
				},
				ctx: ctx,
			},
			expected: assert.AnError,
		},
		{
			name: "ctx timeout",
			input: input{
				ctxName: "https://okteto.com",
				ns:      "test",
				currentStore: &okteto.OktetoContextStore{
					Contexts: map[string]*okteto.OktetoContext{
						"https://okteto.com": {
							IsOkteto: true,
						},
						"k8s_cluster": {},
					},
					CurrentContext: "https://okteto.com",
				},
				ctx: cancelledCtx,
				oktetoClientProvider: fakeOktetoClientProvider{
					err: assert.AnError,
				},
			},
			expected: context.Canceled,
		},
		{
			name: "success",
			input: input{
				ctxName: "https://okteto.com",
				ns:      "test",
				ctx:     ctx,
				currentStore: &okteto.OktetoContextStore{
					Contexts: map[string]*okteto.OktetoContext{
						"https://okteto.com": {
							IsOkteto: true,
						},
						"k8s_cluster": {},
					},
					CurrentContext: "https://okteto.com",
				},
				oktetoClientProvider: fakeOktetoClientProvider{
					client: &client.FakeOktetoClient{
						KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{
							Token: types.KubeTokenResponse{},
						}),
					},
				},
			},
			expected: nil,
		},
	}
	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			v := newPreReqValidator(
				withCtxName(tc.input.ctxName),
				withNamespace(tc.input.ns),
				withK8sClientProvider(tc.input.k8sClientProvider),
				withOktetoClientProvider(tc.input.oktetoClientProvider),
			)
			v.getContextStore = func() *okteto.OktetoContextStore {
				return tc.input.currentStore
			}
			v.getCtxResource = func(s1, s2 string) *contextCMD.ContextOptions {
				return &contextCMD.ContextOptions{
					Context:   s1,
					Namespace: s2,
				}
			}
			err := v.validate(tc.input.ctx)
			assert.ErrorIs(t, err, tc.expected)
		})
	}
}

func TestCtxValidator(t *testing.T) {
	type input struct {
		ctxName           string
		k8sClientProvider okteto.K8sClientProvider
		ctx               context.Context
	}

	ctx := context.Background()
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	okteto.CurrentStore = &okteto.OktetoContextStore{
		Contexts: map[string]*okteto.OktetoContext{
			"https://okteto.com": {
				IsOkteto: true,
			},
			"k8s_cluster": {},
		},
	}

	tt := []struct {
		name     string
		input    input
		expected error
	}{
		{
			name: "ctx Name is empty",
			input: input{
				ctxName: "",
				ctx:     ctx,
			},
			expected: errEmptyContext,
		},
		{
			name: "ctxName is not in the list of contexts stored",
			input: input{
				ctxName: "https://not-okteto.com",
				ctx:     ctx,
			},
			expected: errOktetoContextNotFound{ctxName: "https://not-okteto.com"},
		},
		{
			name: "ctxName is a vanilla k8s context",
			input: input{
				ctxName: "k8s_cluster",
				k8sClientProvider: fakeK8sClientProvider{
					client: fake.NewSimpleClientset(),
				},
				ctx: ctx,
			},
			expected: errIsNotOktetoCtx{ctxName: "k8s_cluster"},
		},
		{
			name: "context cancelled",
			input: input{
				ctxName: "https://okteto.com",
				ctx:     cancelledCtx,
			},
			expected: context.Canceled,
		},
		{
			name: "success - using url",
			input: input{
				ctxName: "https://okteto.com",
				ctx:     ctx,
			},
			expected: nil,
		},
		{
			name: "success - using k8s name",
			input: input{
				ctxName: "okteto_com",
				ctx:     ctx,
			},
			expected: nil,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			v := newCtxValidator(&contextCMD.ContextOptions{
				Context: tc.input.ctxName,
			}, tc.input.k8sClientProvider, func() *okteto.OktetoContextStore {
				return okteto.CurrentStore
			})
			err := v.validate(tc.input.ctx)
			assert.ErrorIs(t, err, tc.expected)
		})
	}
}

func TestOktetoKubetokenSupportValidation(t *testing.T) {
	type input struct {
		ctx                      context.Context
		ctxName                  string
		ns                       string
		k8sClient                kubernetes.Interface
		oktetoClient             types.OktetoInterface
		errProvidingOktetoClient error
	}

	ctx := context.Background()
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	tt := []struct {
		name     string
		input    input
		expected error
	}{
		{
			name: "context cancelled",
			input: input{
				ctx:                      cancelledCtx,
				ctxName:                  "https://okteto.cluster.com",
				errProvidingOktetoClient: assert.AnError,
			},
			expected: context.Canceled,
		},
		{
			name: "error providing",
			input: input{
				ctx:                      ctx,
				ctxName:                  "https://okteto.cluster.com",
				errProvidingOktetoClient: assert.AnError,
			},
			expected: assert.AnError,
		},
		{
			name: "another error",
			input: input{
				ctx:     ctx,
				ctxName: "https://okteto.cluster.com",
				oktetoClient: &client.FakeOktetoClient{
					KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{
						Err: assert.AnError,
					}),
				},
			},
			expected: assert.AnError,
		},
		{
			name: "successful from the same ctx",
			input: input{
				ctx:     ctx,
				ctxName: "https://okteto.cluster.com",
				oktetoClient: &client.FakeOktetoClient{
					KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{
						Token: types.KubeTokenResponse{},
					}),
				},
			},
			expected: nil,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			fakeOktetoClientProvider := fakeOktetoClientProvider{
				client: tc.input.oktetoClient,
				err:    tc.input.errProvidingOktetoClient,
			}
			fakeK8sClientProvider := fakeK8sClientProvider{
				client: tc.input.k8sClient,
			}
			ctxResource := &contextCMD.ContextOptions{
				Context:   tc.input.ctxName,
				Namespace: tc.input.ns,
			}
			v := newOktetoSupportValidator(tc.input.ctx, ctxResource, fakeK8sClientProvider, fakeOktetoClientProvider)
			err := v.validate(tc.input.ctx)
			assert.ErrorIs(t, err, tc.expected)
		})
	}
}
