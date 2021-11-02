// Copyright 2021 The Okteto Authors
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
	"context"
	"testing"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	"github.com/okteto/okteto/pkg/okteto"
	"k8s.io/client-go/tools/clientcmd/api"
)

func Test_updateKubeconfig(t *testing.T) {
	ctx := context.Background()

	var tests = []struct {
		name          string
		kubeconfigCtx kubeconfigFields
		context       *okteto.OktetoContextStore
	}{
		{
			name: "change current ctx",
			kubeconfigCtx: kubeconfigFields{
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
		},
		{
			name: "change current namespace",
			kubeconfigCtx: kubeconfigFields{
				Name:           []string{"to-change"},
				Namespace:      []string{"to-change"},
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
		},
		{
			name:          "create if it does not exists",
			kubeconfigCtx: kubeconfigFields{},
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
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			okteto.CurrentStore = tt.context
			if len(tt.kubeconfigCtx.Name) > 0 {
				createKubeconfig(tt.kubeconfigCtx)
			}

			if err := ExecuteUpdateKubeconfig(ctx); err != nil {
				t.Fatal(err)
			}
			cfg := kubeconfig.Get(config.GetKubeconfigPath())
			if cfg == nil {
				t.Fatal("not written cfg")
			}
			if cfg.CurrentContext != tt.context.CurrentContext {
				t.Fatal("Not updated correctly")
			}
			if cfg.Contexts[tt.context.CurrentContext].Namespace != tt.context.Contexts[tt.context.CurrentContext].Namespace {
				t.Fatal("not updated correctly")
			}
		})
	}
}
