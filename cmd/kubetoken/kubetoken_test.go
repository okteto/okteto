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
	"log"
	"os"
	"testing"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/internal/test/client"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/okteto/okteto/pkg/vars"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type varManagerLogger struct{}

func (varManagerLogger) Yellow(_ string, _ ...interface{}) {}
func (varManagerLogger) AddMaskedWord(_ string)            {}

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
	fakeCtx *okteto.ContextStateless
	err     error
}

func (f fakeCtxCmdRunner) RunStateless(ctx context.Context, ctxOptions *contextCMD.Options) (*okteto.ContextStateless, error) {
	return f.fakeCtx, f.err
}

func TestMain(m *testing.M) {
	varManager := vars.NewVarsManager(&varManagerLogger{})
	tmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		log.Fatal(err)
	}
	defer func(path string) {
		err := os.RemoveAll(path)
		if err != nil {
			log.Fatal(err)
		}
	}(tmpDir)

	varManager.AddLocalVar("HOME", tmpDir)
	vars.GlobalVarManager = varManager

	exitCode := m.Run()

	os.Exit(exitCode)
}

func TestKubetoken(t *testing.T) {
	ctx := context.Background()
	type input struct {
		fakeOktetoClientProvider fakeOktetoClientProvider
		fakeCtxCmdRunner         fakeCtxCmdRunner
		flags                    Flags
	}

	fakeCtxStore := &okteto.ContextStateless{
		Store: &okteto.ContextStore{
			CurrentContext: "https://okteto.dev",
			Contexts: map[string]*okteto.Context{
				"https://okteto.dev": {
					IsOkteto: true,
				},
			},
		},
	}

	tt := []struct {
		expected error
		input    input
		name     string
	}{
		{
			name: "error on validation",
			input: input{
				flags: Flags{
					Context: "",
				},
			},
			expected: errEmptyContext,
		},
		{
			name: "error on context command Run",
			input: input{
				flags: Flags{
					Context: "https://okteto.dev",
				},
				fakeOktetoClientProvider: fakeOktetoClientProvider{
					client: &client.FakeOktetoClient{
						KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{
							Token: types.KubeTokenResponse{},
						}),
					},
				},
				fakeCtxCmdRunner: fakeCtxCmdRunner{
					fakeCtx: fakeCtxStore,
					err:     assert.AnError,
				},
			},
			expected: assert.AnError,
		},
		{
			name: "error getting kubetoken",
			input: input{
				flags: Flags{
					Context: "https://okteto.dev",
				},
				fakeOktetoClientProvider: fakeOktetoClientProvider{
					client: &client.FakeOktetoClient{
						KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{
							Err: assert.AnError,
						}),
					},
				},
				fakeCtxCmdRunner: fakeCtxCmdRunner{
					fakeCtx: fakeCtxStore,
					err:     assert.AnError,
				},
			},
			expected: assert.AnError,
		},
		{
			name: "successful",
			input: input{
				flags: Flags{
					Context:   "https://okteto.dev",
					Namespace: "namespace",
				},
				fakeOktetoClientProvider: fakeOktetoClientProvider{
					client: &client.FakeOktetoClient{
						KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{
							Token: types.KubeTokenResponse{},
						}),
					},
				},
				fakeCtxCmdRunner: fakeCtxCmdRunner{
					fakeCtx: fakeCtxStore,
				},
			},
			expected: nil,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			varManager := vars.NewVarsManager(&varManagerLogger{})

			cmd := NewKubetokenCmd(varManager)
			cmd.oktetoClientProvider = tc.input.fakeOktetoClientProvider
			cmd.oktetoCtxCmdRunner = tc.input.fakeCtxCmdRunner
			cmd.ctxStore = fakeCtxStore.Store
			cmd.initCtxFunc = func(string, string, *vars.Manager) *contextCMD.Options {
				return &contextCMD.Options{
					Context:   tc.input.flags.Context,
					Namespace: tc.input.flags.Namespace,
				}
			}

			log.Printf("running test %s", tc.name)
			err := cmd.Run(ctx, tc.input.flags)
			assert.ErrorIs(t, err, tc.expected)
		})
	}
}
