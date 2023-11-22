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
package model

import (
	"os"
	"reflect"
	"testing"
)

func Test_GetContextResource(t *testing.T) {
	var tests = []struct {
		env      map[string]string
		want     *ContextResource
		name     string
		manifest []byte
	}{
		{
			name:     "empty",
			manifest: []byte("name: test"),
			want:     &ContextResource{},
		},
		{
			name:     "context-and-namespace",
			manifest: []byte("name: test\ncontext: context\nnamespace: namespace"),
			want:     &ContextResource{Context: "context", Namespace: "namespace"},
		},
		{
			name:     "envvars",
			manifest: []byte("name: test\ncontext: context-${CONTEXT}\nnamespace: namespace-${NAMESPACE}"),
			env:      map[string]string{"CONTEXT": "test1", "NAMESPACE": "test2"},
			want:     &ContextResource{Context: "context-test1", Namespace: "namespace-test2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "manifest")
			if err != nil {
				t.Fatalf("failed to create dynamic manifest file: %s", err.Error())
			}
			if err := os.WriteFile(tmpFile.Name(), tt.manifest, 0600); err != nil {
				t.Fatalf("failed to write manifest file: %s", err.Error())
			}
			defer os.RemoveAll(tmpFile.Name())

			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			result, err := GetContextResource(tmpFile.Name())
			if err != nil {
				t.Fatalf("error reading manifest: %v", err)
			}
			if !reflect.DeepEqual(tt.want, result) {
				t.Errorf("Test '%s' failed: %+v", tt.name, result)
			}
		})
	}
}

func Test_UpdateNamespace(t *testing.T) {
	var tests = []struct {
		in        *ContextResource
		out       *ContextResource
		name      string
		namespace string
		wantError bool
	}{
		{
			name:      "all-empty",
			in:        &ContextResource{},
			namespace: "",
			out:       &ContextResource{},
			wantError: false,
		},
		{
			name:      "namespace-in-manifest-ns-empty",
			in:        &ContextResource{Namespace: "ns-manifest"},
			namespace: "",
			out:       &ContextResource{Namespace: "ns-manifest"},
			wantError: false,
		},
		{
			name:      "namespace-in-manifest-with-same-arg",
			in:        &ContextResource{Namespace: "ns-manifest"},
			namespace: "ns-manifest",
			out:       &ContextResource{Namespace: "ns-manifest"},
			wantError: false,
		},
		{
			name:      "namespace-in-manifest-with-different-arg",
			in:        &ContextResource{Namespace: "ns-manifest"},
			namespace: "ns-arg",
			wantError: true,
		},
		{
			name:      "no-namespace-in-manifest-with-arg",
			in:        &ContextResource{},
			namespace: "ns-arg",
			out:       &ContextResource{Namespace: "ns-arg"},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.in.UpdateNamespace(tt.namespace)
			if err != nil && !tt.wantError {
				t.Errorf("Test '%s' failed: %+v", tt.name, err)
			}
			if err == nil && tt.wantError {
				t.Errorf("Test '%s' didn't failed", tt.name)
			}
			if err != nil {
				return
			}
			if !reflect.DeepEqual(tt.in, tt.out) {
				t.Errorf("Test '%s' failed: %+v", tt.name, tt.in)
			}
		})
	}
}

func Test_UpdateContext(t *testing.T) {
	var tests = []struct {
		in        *ContextResource
		out       *ContextResource
		name      string
		context   string
		wantError bool
	}{
		{
			name:      "all-empty",
			in:        &ContextResource{},
			context:   "",
			out:       &ContextResource{},
			wantError: false,
		},
		{
			name:      "context-in-manifest-rest-empty",
			in:        &ContextResource{Context: "ctx-manifest"},
			context:   "",
			out:       &ContextResource{Context: "ctx-manifest"},
			wantError: false,
		},
		{
			name:      "context-in-manifest-with-same-arg",
			in:        &ContextResource{Context: "ctx-manifest"},
			context:   "ctx-manifest",
			out:       &ContextResource{Context: "ctx-manifest"},
			wantError: false,
		},
		{
			name:      "context-in-manifest-with-different-arg",
			in:        &ContextResource{Context: "ctx-manifest"},
			context:   "ctx-arg",
			wantError: true,
		},
		{
			name:      "no-context-in-manifest-with-arg",
			in:        &ContextResource{},
			context:   "ctx-arg",
			out:       &ContextResource{Context: "ctx-arg"},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.in.UpdateContext(tt.context)
			if err != nil && !tt.wantError {
				t.Errorf("Test '%s' failed: %+v", tt.name, err)
			}
			if err == nil && tt.wantError {
				t.Errorf("Test '%s' didn't failed", tt.name)
			}
			if err != nil {
				return
			}
			if !reflect.DeepEqual(tt.in, tt.out) {
				t.Errorf("Test '%s' failed: %+v", tt.name, tt.in)
			}
		})
	}
}
