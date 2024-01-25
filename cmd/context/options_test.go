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
	"reflect"
	"testing"

	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/assert"
)

func Test_initFromContext(t *testing.T) {
	var tests = []struct {
		in       *Options
		ctxStore *okteto.ContextStore
		want     *Options
		name     string
	}{
		{
			name:     "all-empty",
			in:       &Options{},
			ctxStore: &okteto.ContextStore{},
			want:     &Options{},
		},
		{
			name: "all-empty-and-wrong-current-context",
			in:   &Options{},
			ctxStore: &okteto.ContextStore{
				CurrentContext: "bad",
			},
			want: &Options{},
		},
		{
			name: "from-options",
			in: &Options{
				Context:   "ctx-from-opts",
				Namespace: "ns-from-opts",
			},
			ctxStore: &okteto.ContextStore{
				CurrentContext: "context",
				Contexts: map[string]*okteto.Context{
					"context": {
						Name:      "context",
						Namespace: "namespace",
					},
				},
			},
			want: &Options{
				Context:   "ctx-from-opts",
				Namespace: "ns-from-opts",
			},
		},
		{
			name: "from-context",
			in:   &Options{},
			ctxStore: &okteto.ContextStore{
				CurrentContext: "context",
				Contexts: map[string]*okteto.Context{
					"context": {
						Name:      "context",
						Namespace: "namespace",
					},
				},
			},
			want: &Options{
				Context:   "context",
				Namespace: "namespace",
			},
		},
		{
			name: "from-context-namespace-from-options",
			in: &Options{
				Namespace: "ns-from-opts",
			},
			ctxStore: &okteto.ContextStore{
				CurrentContext: "context",
				Contexts: map[string]*okteto.Context{
					"context": {
						Name:      "context",
						Namespace: "namespace",
					},
				},
			},
			want: &Options{
				Context:   "context",
				Namespace: "ns-from-opts",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			okteto.CurrentStore = tt.ctxStore
			tt.in.InitFromContext()
			if !reflect.DeepEqual(tt.in, tt.want) {
				t.Errorf("Test '%s' failed: %+v", tt.name, tt.in)
			}
		})
	}
}

func Test_initFromEnvVars(t *testing.T) {
	var tests = []struct {
		in   *Options
		env  map[string]string
		want *Options
		name string
	}{
		{
			name: "all-empty",
			in:   &Options{},
			env: map[string]string{
				"OKTETO_URL":       "",
				"OKTETO_CONTEXT":   "",
				"OKTETO_TOKEN":     "",
				"OKTETO_NAMESPACE": "",
			},
			want: &Options{},
		},
		{
			name: "token-in-options-no-context",
			in: &Options{
				Token: "token",
			},
			env: map[string]string{
				"OKTETO_URL":       "",
				"OKTETO_CONTEXT":   "",
				"OKTETO_TOKEN":     "",
				"OKTETO_NAMESPACE": "",
			},
			want: &Options{
				Token:    "token",
				Context:  okteto.CloudURL,
				IsOkteto: true,
			},
		},
		{
			name: "token-in-options-with-envar",
			in: &Options{
				Token: "token",
			},
			env: map[string]string{
				"OKTETO_URL":       "",
				"OKTETO_CONTEXT":   "",
				"OKTETO_TOKEN":     "bad-token",
				"OKTETO_NAMESPACE": "",
			},
			want: &Options{
				Token:    "token",
				Context:  okteto.CloudURL,
				IsOkteto: true,
			},
		},
		{
			name: "token-notin-options-with-envar",
			in:   &Options{},
			env: map[string]string{
				"OKTETO_URL":       "",
				"OKTETO_CONTEXT":   "",
				"OKTETO_TOKEN":     "token",
				"OKTETO_NAMESPACE": "",
			},
			want: &Options{
				Token:         "token",
				Context:       okteto.CloudURL,
				IsOkteto:      true,
				InferredToken: true,
			},
		},
		{
			name: "context-in-options-no-envar",
			in: &Options{
				Context: "context",
			},
			env: map[string]string{
				"OKTETO_URL":       "",
				"OKTETO_CONTEXT":   "",
				"OKTETO_TOKEN":     "",
				"OKTETO_NAMESPACE": "",
			},
			want: &Options{
				Context: "context",
			},
		},
		{
			name: "context-in-options-with-envar",
			in: &Options{
				Context: "context",
			},
			env: map[string]string{
				"OKTETO_URL":       "okteto-url",
				"OKTETO_CONTEXT":   "okteto-context",
				"OKTETO_TOKEN":     "",
				"OKTETO_NAMESPACE": "",
			},
			want: &Options{
				Context: "context",
			},
		},
		{
			name: "context-notin-options-with-envar-context",
			in:   &Options{},
			env: map[string]string{
				"OKTETO_URL":       "",
				"OKTETO_CONTEXT":   "okteto-context",
				"OKTETO_TOKEN":     "",
				"OKTETO_NAMESPACE": "",
			},
			want: &Options{
				Context: "okteto-context",
			},
		},
		{
			name: "context-notin-options-with-envar-url",
			in:   &Options{},
			env: map[string]string{
				"OKTETO_URL":       "okteto-url",
				"OKTETO_CONTEXT":   "",
				"OKTETO_TOKEN":     "",
				"OKTETO_NAMESPACE": "",
			},
			want: &Options{
				Context:  "okteto-url",
				IsOkteto: true,
			},
		},
		{
			name: "context-notin-options-with-token-in-options-and-with-envar-url",
			in: &Options{
				Token: "token",
			},
			env: map[string]string{
				"OKTETO_URL":       "okteto-url",
				"OKTETO_CONTEXT":   "",
				"OKTETO_TOKEN":     "",
				"OKTETO_NAMESPACE": "",
			},
			want: &Options{
				Token:    "token",
				Context:  "okteto-url",
				IsOkteto: true,
			},
		},
		{
			name: "context-notin-options-and-with-envar-url-and-token",
			in:   &Options{},
			env: map[string]string{
				"OKTETO_URL":       "okteto-url",
				"OKTETO_CONTEXT":   "",
				"OKTETO_TOKEN":     "token-envvar",
				"OKTETO_NAMESPACE": "",
			},
			want: &Options{
				Token:         "token-envvar",
				Context:       "okteto-url",
				IsOkteto:      true,
				InferredToken: true,
			},
		},
		{
			name: "namespace-in-options-no-envar",
			in: &Options{
				Namespace: "namespace",
			},
			env: map[string]string{
				"OKTETO_URL":       "",
				"OKTETO_CONTEXT":   "",
				"OKTETO_TOKEN":     "",
				"OKTETO_NAMESPACE": "",
			},
			want: &Options{
				Namespace: "namespace",
			},
		},
		{
			name: "namespace-in-options-with-envar",
			in: &Options{
				Namespace: "namespace",
			},
			env: map[string]string{
				"OKTETO_URL":       "",
				"OKTETO_CONTEXT":   "",
				"OKTETO_TOKEN":     "",
				"OKTETO_NAMESPACE": "okteto-ns",
			},
			want: &Options{
				Namespace: "namespace",
			},
		},
		{
			name: "namespace-notin-options-with-envar",
			in:   &Options{},
			env: map[string]string{
				"OKTETO_URL":       "",
				"OKTETO_CONTEXT":   "",
				"OKTETO_TOKEN":     "",
				"OKTETO_NAMESPACE": "okteto-ns",
			},
			want: &Options{
				Namespace: "okteto-ns",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			tt.in.InitFromEnvVars()
			assert.Equal(t, tt.in, tt.want)
		})
	}
}
