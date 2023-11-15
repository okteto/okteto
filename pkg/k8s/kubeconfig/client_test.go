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

package kubeconfig

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type kubeconfigFields struct {
	CurrentContext string
	Name           []string
	Namespace      []string
}

func TestGetKubeconfig(t *testing.T) {
	var tests = []struct {
		expected         *clientcmdapi.Config
		name             string
		KubeconfigFields []kubeconfigFields
	}{
		{
			name: "only one",
			KubeconfigFields: []kubeconfigFields{
				{
					Name:           []string{"test"},
					Namespace:      []string{"test"},
					CurrentContext: "test",
				},
			},
			expected: &clientcmdapi.Config{
				Contexts: map[string]*clientcmdapi.Context{
					"test": {
						Namespace:  "test",
						Extensions: map[string]runtime.Object{},
					},
				},
				CurrentContext: "test",
			},
		},
		{
			name: "two config files different contexts",
			KubeconfigFields: []kubeconfigFields{
				{
					Name:           []string{"test"},
					Namespace:      []string{"test"},
					CurrentContext: "test",
				},
				{
					Name:           []string{"foo"},
					Namespace:      []string{"bar"},
					CurrentContext: "foo",
				},
			},
			expected: &clientcmdapi.Config{
				Contexts: map[string]*clientcmdapi.Context{
					"test": {
						Namespace:  "test",
						Extensions: map[string]runtime.Object{},
					},
					"foo": {
						Namespace:  "bar",
						Extensions: map[string]runtime.Object{},
					},
				},
				CurrentContext: "test",
			},
		},
		{
			name: "two config files overlap contexts",
			KubeconfigFields: []kubeconfigFields{
				{
					Name:           []string{"test"},
					Namespace:      []string{"test"},
					CurrentContext: "test",
				},
				{
					Name:           []string{"test"},
					Namespace:      []string{"namespace"},
					CurrentContext: "test",
				},
			},
			expected: &clientcmdapi.Config{
				Contexts: map[string]*clientcmdapi.Context{
					"test": {
						Namespace:  "namespace",
						Extensions: map[string]runtime.Object{},
					},
				},
				CurrentContext: "test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dirs := make([]string, 0)
			for _, kubeconfig := range tt.KubeconfigFields {
				dir, err := createKubeconfig(kubeconfig)
				if err != nil {
					t.Fatal(err)
				}
				dirs = append(dirs, dir)
				defer os.Remove(dir)
			}
			config := Get(dirs)

			for _, c := range config.Contexts {
				c.LocationOfOrigin = ""
			}

			assert.Equal(t, config.Contexts, tt.expected.Contexts)
			assert.Equal(t, config.CurrentContext, tt.expected.CurrentContext)
		})
	}
}

func createKubeconfig(kubeconfigFields kubeconfigFields) (string, error) {
	dir, err := os.CreateTemp("", "")
	if err != nil {
		return "", err
	}

	contexts := make(map[string]*clientcmdapi.Context)
	for idx := range kubeconfigFields.Name {
		contexts[kubeconfigFields.Name[idx]] = &clientcmdapi.Context{Namespace: kubeconfigFields.Namespace[idx]}
	}
	cfg := &clientcmdapi.Config{
		Contexts:       contexts,
		CurrentContext: kubeconfigFields.CurrentContext,
	}
	if err := Write(cfg, dir.Name()); err != nil {
		return "", err
	}
	return dir.Name(), nil
}
