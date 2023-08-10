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
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/tools/clientcmd/api"
)

func Test_ExecuteUpdateKubeconfig(t *testing.T) {

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
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			okteto.CurrentStore = tt.context
			file, err := test.CreateKubeconfig(tt.kubeconfigCtx)
			if err != nil {
				assert.NoError(t, err, "error creating temporal kubeconfig")
			}
			defer os.Remove(file)

			okContext := okteto.Context()
			kubeconfigPaths := []string{file}

			err = ExecuteUpdateKubeconfig(okContext, kubeconfigPaths, false)
			assert.NoError(t, err, "error writing kubeconfig")

			cfg := kubeconfig.Get(kubeconfigPaths)
			assert.NotNil(t, cfg, "kubeconfig is nil")
			assert.Equal(t, tt.context.CurrentContext, cfg.CurrentContext, "current context has changed")
			assert.Equal(t, tt.context.Contexts[tt.context.CurrentContext].Namespace, cfg.Contexts[tt.context.CurrentContext].Namespace, "namespace has changed")

		})
	}
}
