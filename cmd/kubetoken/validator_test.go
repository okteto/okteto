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

	"github.com/okteto/okteto/internal/test/client"
	"github.com/okteto/okteto/pkg/k8s/ingresses"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
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
func (f fakeK8sClientProvider) GetIngressClient() (*ingresses.Client, error) {
	return nil, nil
}

type fakeOktetoClientProvider struct {
	client types.OktetoInterface
	err    error
}

func (f fakeOktetoClientProvider) Provide() (types.OktetoInterface, error) {
	return f.client, f.err
}

func TestPreReqValidator(t *testing.T) {
	type input struct {
		ctxName           string
		ns                string
		k8sClientProvider okteto.K8sClientProvider
		ctx               context.Context
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
			},
			expected: errEmptyContext,
		},
		{
			name: "fail on namespace validation",
			input: input{
				ctxName: "https://okteto.com",
				ns:      "",
				ctx:     ctx,
			},
			expected: errEmptyNamespace,
		},
		{
			name: "fail on okteto client validation",
			input: input{
				ctxName:           "https://okteto.com",
				ns:                "test",
				k8sClientProvider: fakeK8sClientProvider{err: assert.AnError},
				ctx:               ctx,
			},
			expected: assert.AnError,
		},
		{
			name: "ctx timeout",
			input: input{
				ctxName: "https://okteto.com",
				ns:      "test",
				ctx:     cancelledCtx,
			},
			expected: context.Canceled,
		},
		{
			name: "success",
			input: input{
				ctxName: "https://okteto.com",
				ns:      "test",
				ctx:     ctx,
			},
			expected: context.Canceled,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			v := newPreReqValidator(
				withCtxName(tc.input.ctxName),
				withNamespace(tc.input.ns),
				withK8sClientProvider(tc.input.k8sClientProvider),
			)
			err := v.Validate(context.Background())
			assert.Equal(t, tc.expected, err)
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
			v := newCtxValidator(tc.input.ctxName, tc.input.k8sClientProvider)
			err := v.Validate(tc.input.ctx)
			assert.ErrorIs(t, err, tc.expected)
		})
	}
}

func TestNsValidator(t *testing.T) {
	type input struct {
		ns           string
		oktetoClient types.OktetoInterface
		ctx          context.Context
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
				ns: "test",
				oktetoClient: &client.FakeOktetoClient{
					Namespace: client.NewFakeNamespaceClient([]types.Namespace{}, nil),
					Preview:   client.NewFakePreviewClient(&client.FakePreviewResponse{}),
				},
				ctx: cancelledCtx,
			},
			expected: context.Canceled,
		},
		{
			name: "ns Name is empty",
			input: input{
				ns:  "",
				ctx: ctx,
			},
			expected: errEmptyNamespace,
		},
		{
			name: "ns not found",
			input: input{
				ns: "test",
				oktetoClient: &client.FakeOktetoClient{
					Namespace: client.NewFakeNamespaceClient([]types.Namespace{}, nil),
					Preview:   client.NewFakePreviewClient(&client.FakePreviewResponse{}),
				},
				ctx: ctx,
			},
			expected: errNamespaceForbidden{ns: "test"},
		},
		{
			name: "error retrieving ns",
			input: input{
				ns: "test",
				oktetoClient: &client.FakeOktetoClient{
					Namespace: client.NewFakeNamespaceClient([]types.Namespace{}, assert.AnError),
					Preview:   client.NewFakePreviewClient(&client.FakePreviewResponse{}),
				},
				ctx: ctx,
			},
			expected: assert.AnError,
		},
		{
			name: "error retrieving preview",
			input: input{
				ns: "test",
				oktetoClient: &client.FakeOktetoClient{
					Namespace: client.NewFakeNamespaceClient([]types.Namespace{}, nil),
					Preview: client.NewFakePreviewClient(&client.FakePreviewResponse{
						ErrList: assert.AnError,
					}),
				},
				ctx: ctx,
			},
			expected: assert.AnError,
		},
		{
			name: "success ns is a namespace",
			input: input{
				ns: "test",
				oktetoClient: &client.FakeOktetoClient{
					Namespace: client.NewFakeNamespaceClient([]types.Namespace{{ID: "test"}}, nil),
					Preview:   client.NewFakePreviewClient(&client.FakePreviewResponse{}),
				},
				ctx: ctx,
			},
			expected: nil,
		},
		{
			name: "success - ns is a preview",
			input: input{
				ns: "test",
				oktetoClient: &client.FakeOktetoClient{
					Namespace: client.NewFakeNamespaceClient([]types.Namespace{}, nil),
					Preview: client.NewFakePreviewClient(&client.FakePreviewResponse{
						PreviewList: []types.Preview{{ID: "test"}},
					}),
				},
				ctx: ctx,
			},
			expected: nil,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			v := newNsValidator(tc.input.ns, fakeOktetoClientProvider{tc.input.oktetoClient, nil})
			err := v.Validate(tc.input.ctx)
			assert.ErrorIs(t, err, tc.expected)
		})
	}
}

func TestK8sTokenRequestValidator(t *testing.T) {
	type input struct {
		ctx         context.Context
		k8sClient   kubernetes.Interface
		providerErr error
	}

	ctx := context.Background()
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	fakeK8sClientWithTokenRequest := fake.NewSimpleClientset()
	fakeK8sClientWithTokenRequest.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "authentication.k8s.io/v1",
			APIResources: []metav1.APIResource{
				{
					Kind: "TokenRequest",
				},
			},
		},
	}
	fakeK8sClientWithoutTokenRequest := fake.NewSimpleClientset()
	fakeK8sClientWithoutTokenRequest.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "authentication.k8s.io/v1",
			APIResources: []metav1.APIResource{},
		},
	}

	tt := []struct {
		name     string
		input    input
		expected error
	}{
		{
			name: "context cancelled",
			input: input{
				ctx:         cancelledCtx,
				providerErr: assert.AnError,
			},
			expected: context.Canceled,
		},
		{
			name: "error provider",
			input: input{
				ctx:         ctx,
				providerErr: assert.AnError,
			},
			expected: assert.AnError,
		},
		{
			name: "not have tokenrequest",
			input: input{
				ctx:       ctx,
				k8sClient: fake.NewSimpleClientset(),
			},
			expected: errTokenRequestNotSupported,
		},
		{
			name: "k8s doesn't support tokenrequest",
			input: input{
				ctx:       ctx,
				k8sClient: fakeK8sClientWithoutTokenRequest,
			},
			expected: errTokenRequestNotSupported,
		},
		{
			name: "success",
			input: input{
				ctx:       ctx,
				k8sClient: fakeK8sClientWithTokenRequest,
			},
			expected: nil,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			fakeK8sClientProvider := fakeK8sClientProvider{
				client: tc.input.k8sClient,
				err:    tc.input.providerErr,
			}
			v := newK8sSupportValidator(fakeK8sClientProvider)
			v.getApiConfig = func() *clientcmdapi.Config {
				return &clientcmdapi.Config{}
			}
			err := v.Validate(tc.input.ctx)
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
			v := newOktetoSupportValidator(tc.input.ctx, tc.input.ctxName, tc.input.ns, fakeK8sClientProvider, fakeOktetoClientProvider)
			err := v.Validate(tc.input.ctx)
			assert.ErrorIs(t, err, tc.expected)
		})
	}
}
