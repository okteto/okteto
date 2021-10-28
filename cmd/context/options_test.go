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
	"os"
	"reflect"
	"testing"

	"github.com/okteto/okteto/pkg/okteto"
)

func Test_initFromContext(t *testing.T) {
	var tests = []struct {
		name     string
		in       *ContextOptions
		ctxStore *okteto.OktetoContextStore
		want     *ContextOptions
	}{
		{
			name:     "all-empty",
			in:       &ContextOptions{},
			ctxStore: &okteto.OktetoContextStore{},
			want:     &ContextOptions{},
		},
		{
			name: "all-empty-and-wrong-current-context",
			in:   &ContextOptions{},
			ctxStore: &okteto.OktetoContextStore{
				CurrentContext: "bad",
			},
			want: &ContextOptions{},
		},
		{
			name: "from-options",
			in: &ContextOptions{
				Context:   "ctx-from-opts",
				Namespace: "ns-from-opts",
			},
			ctxStore: &okteto.OktetoContextStore{
				CurrentContext: "context",
				Contexts: map[string]*okteto.OktetoContext{
					"context": {
						Name:      "context",
						Namespace: "namespace",
					},
				},
			},
			want: &ContextOptions{
				Context:   "ctx-from-opts",
				Namespace: "ns-from-opts",
			},
		},
		{
			name: "from-context",
			in:   &ContextOptions{},
			ctxStore: &okteto.OktetoContextStore{
				CurrentContext: "context",
				Contexts: map[string]*okteto.OktetoContext{
					"context": {
						Name:      "context",
						Namespace: "namespace",
					},
				},
			},
			want: &ContextOptions{
				Context:   "context",
				Namespace: "namespace",
			},
		},
		{
			name: "from-context-namespace-from-options",
			in: &ContextOptions{
				Namespace: "ns-from-opts",
			},
			ctxStore: &okteto.OktetoContextStore{
				CurrentContext: "context",
				Contexts: map[string]*okteto.OktetoContext{
					"context": {
						Name:      "context",
						Namespace: "namespace",
					},
				},
			},
			want: &ContextOptions{
				Context:   "context",
				Namespace: "ns-from-opts",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			okteto.CurrentStore = tt.ctxStore
			tt.in.initFromContext()
			if !reflect.DeepEqual(tt.in, tt.want) {
				t.Errorf("Test '%s' failed: %+v", tt.name, tt.in)
			}
		})
	}
}

func Test_initFromEnvVars(t *testing.T) {
	var tests = []struct {
		name string
		in   *ContextOptions
		env  map[string]string
		want *ContextOptions
	}{
		{
			name: "all-empty",
			in:   &ContextOptions{},
			want: &ContextOptions{},
		},
		{
			name: "token-in-options-no-context",
			in: &ContextOptions{
				Token: "token",
			},
			want: &ContextOptions{
				Token:    "token",
				Context:  okteto.CloudURL,
				isOkteto: true,
			},
		},
		{
			name: "token-in-options-with-envar",
			in: &ContextOptions{
				Token: "token",
			},
			env: map[string]string{"OKTETO_TOKEN": "bad-token"},
			want: &ContextOptions{
				Token:    "token",
				Context:  okteto.CloudURL,
				isOkteto: true,
			},
		},
		{
			name: "token-notin-options-with-envar",
			in:   &ContextOptions{},
			env:  map[string]string{"OKTETO_TOKEN": "token"},
			want: &ContextOptions{
				Token:    "token",
				Context:  okteto.CloudURL,
				isOkteto: true,
			},
		},
		{
			name: "context-in-options-no-envar",
			in: &ContextOptions{
				Context: "context",
			},
			want: &ContextOptions{
				Context: "context",
			},
		},
		{
			name: "context-in-options-with-envar",
			in: &ContextOptions{
				Context: "context",
			},
			env: map[string]string{"OKTETO_URL": "okteto-url", "OKTETO_CONTEXT": "okteto-context"},
			want: &ContextOptions{
				Context: "context",
			},
		},
		{
			name: "context-notin-options-with-envar-context",
			in:   &ContextOptions{},
			env:  map[string]string{"OKTETO_CONTEXT": "okteto-context"},
			want: &ContextOptions{
				Context: "okteto-context",
			},
		},
		{
			name: "context-notin-options-with-envar-url",
			in:   &ContextOptions{},
			env:  map[string]string{"OKTETO_URL": "okteto-url"},
			want: &ContextOptions{
				Context:  "okteto-url",
				isOkteto: true,
			},
		},
		{
			name: "context-notin-options-with-token-in-options-and-with-envar-url",
			in: &ContextOptions{
				Token: "token",
			},
			env: map[string]string{"OKTETO_URL": "okteto-url"},
			want: &ContextOptions{
				Token:    "token",
				Context:  "okteto-url",
				isOkteto: true,
			},
		},
		{
			name: "context-notin-options-and-with-envar-url-and-token",
			in:   &ContextOptions{},
			env:  map[string]string{"OKTETO_URL": "okteto-url", "OKTETO_TOKEN": "token-envvar"},
			want: &ContextOptions{
				Token:    "token-envvar",
				Context:  "okteto-url",
				isOkteto: true,
			},
		},
		{
			name: "namespace-in-options-no-envar",
			in: &ContextOptions{
				Namespace: "namespace",
			},
			want: &ContextOptions{
				Namespace: "namespace",
			},
		},
		{
			name: "namespace-in-options-with-envar",
			in: &ContextOptions{
				Namespace: "namespace",
			},
			env: map[string]string{"OKTETO_NAMESPACE": "okteto-ns"},
			want: &ContextOptions{
				Namespace: "namespace",
			},
		},
		{
			name: "namespace-notin-options-with-envar",
			in:   &ContextOptions{},
			env:  map[string]string{"OKTETO_NAMESPACE": "okteto-ns"},
			want: &ContextOptions{
				Namespace: "okteto-ns",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				os.Setenv(k, v)
			}
			tt.in.initFromEnvVars()
			if !reflect.DeepEqual(tt.in, tt.want) {
				t.Errorf("Test '%s' failed: %+v", tt.name, tt.in)
			}
			for k := range tt.env {
				os.Unsetenv(k)
			}
		})
	}
}
