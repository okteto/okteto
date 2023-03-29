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

package okteto

import (
	"context"
	"testing"

	"github.com/okteto/okteto/internal/test"
)

func Test_UrlToKubernetesContext(t *testing.T) {
	var tests = []struct {
		name string
		in   string
		want string
	}{
		{name: "is-url-with-protocol", in: "https://cloud.okteto.com", want: "cloud_okteto_com"},
		{name: "is-k8scontext", in: "minikube", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if result := UrlToKubernetesContext(tt.in); result != tt.want {
				t.Errorf("Test '%s' failed: %s", tt.name, result)
			}
		})
	}
}

func Test_K8sContextToOktetoUrl(t *testing.T) {
	var tests = []struct {
		name string
		in   string
		want string
	}{
		{name: "is-url", in: CloudURL, want: CloudURL},
		{name: "is-okteto-context", in: "cloud_okteto_com", want: CloudURL},
		{name: "is-empty", in: "", want: ""},
		{name: "is-k8scontext", in: "minikube", want: "minikube"},
	}

	CurrentStore = &OktetoContextStore{
		Contexts: map[string]*OktetoContext{CloudURL: {IsOkteto: true}},
	}
	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeK8sProvider := test.NewFakeK8sProvider()
			if result := K8sContextToOktetoUrl(ctx, tt.in, "namespace", fakeK8sProvider); result != tt.want {
				t.Errorf("Test '%s' failed: %s", tt.name, result)
			}
		})
	}
}

func Test_IsOktetoCloud(t *testing.T) {
	var tests = []struct {
		name    string
		context *OktetoContext
		want    bool
	}{
		{name: "is-cloud", context: &OktetoContext{Name: "https://cloud.okteto.com"}, want: true},
		{name: "is-staging", context: &OktetoContext{Name: "https://staging.okteto.dev"}, want: true},
		{name: "is-not-cloud", context: &OktetoContext{Name: "https://cindy.okteto.dev"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			CurrentStore = &OktetoContextStore{
				CurrentContext: "test",
				Contexts: map[string]*OktetoContext{
					"test": tt.context,
				},
			}
			if got := IsOktetoCloud(); got != tt.want {
				t.Errorf("IsOktetoCloud, got %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_RemoveSchema(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "https",
			url:  "https://okteto.dev.com",
			want: "okteto.dev.com",
		},
		{
			name: "non url",
			url:  "minikube",
			want: "minikube",
		},
		{
			name: "http",
			url:  "http://okteto.com",
			want: "okteto.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RemoveSchema(tt.url)
			if result != tt.want {
				t.Fatalf("Expected %s but got %s", tt.want, result)
			}
		})
	}
}
