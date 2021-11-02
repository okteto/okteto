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

	"github.com/okteto/okteto/pkg/cmd/login"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type fakeK8sProvider struct {
	objects []runtime.Object
}

func (f *fakeK8sProvider) K8sProvider(clientApiConfig *clientcmdapi.Config) (kubernetes.Interface, *rest.Config, error) {
	return fake.NewSimpleClientset(f.objects...), nil, nil
}

type fakeUserClient struct {
	userContext *okteto.UserContext
}

func (f *fakeUserClient) userClientProvider() (okteto.UserInterface, error) {
	return &okteto.FakeUserClient{UserContext: f.userContext}, nil
}

func Test_createContext(t *testing.T) {
	ctx := context.Background()

	var tests = []struct {
		name          string
		ctxStore      *okteto.OktetoContextStore
		ctxOptions    *ContextOptions
		kubeconfigCtx kubeconfigFields
		expectedErr   bool
		user          *okteto.User
		fakeObjects   []runtime.Object
	}{
		{
			name: "transform k8s to url and create okteto context -> namespace with label",
			ctxStore: &okteto.OktetoContextStore{
				Contexts: map[string]*okteto.OktetoContext{},
			},
			ctxOptions: &ContextOptions{
				isOkteto: false,
				Context:  "cloud_okteto_com",
			},
			kubeconfigCtx: kubeconfigFields{
				Name:      []string{"cloud_okteto_com"},
				Namespace: []string{"test"},
			},
			user: &okteto.User{
				Token: "test",
			},
			fakeObjects: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: v1.ObjectMeta{
						Name: "test",
						Labels: map[string]string{
							model.DevLabel: "true",
						},
						Annotations: map[string]string{
							model.OktetoURLAnnotation: "https://cloud.okteto.com",
						},
					},
				},
			},
			expectedErr: false,
		},
		{
			name: "transform k8s to url and create okteto context no namespace found",
			ctxStore: &okteto.OktetoContextStore{
				Contexts: map[string]*okteto.OktetoContext{},
			},
			ctxOptions: &ContextOptions{
				isOkteto: false,
				Context:  "cloud_okteto_com",
			},
			user: &okteto.User{
				Token: "test",
			},
			kubeconfigCtx: kubeconfigFields{
				Name:      []string{"cloud_okteto_com"},
				Namespace: []string{"test"},
			},
			expectedErr: false,
		},
		{
			name: "transform k8s to url and create okteto context -> namespace without label",
			ctxStore: &okteto.OktetoContextStore{
				Contexts: map[string]*okteto.OktetoContext{},
			},
			ctxOptions: &ContextOptions{
				isOkteto: false,
				Context:  "cloud_okteto_com",
			},
			user: &okteto.User{
				Token: "test",
			},
			kubeconfigCtx: kubeconfigFields{
				Name:      []string{"cloud_okteto_com"},
				Namespace: []string{"test"},
			},
			expectedErr: false,
		},

		{
			name: "transform k8s to url and create okteto context and no namespace defined",
			ctxStore: &okteto.OktetoContextStore{
				Contexts: map[string]*okteto.OktetoContext{},
			},
			ctxOptions: &ContextOptions{
				isOkteto: false,
				Context:  "cloud_okteto_com",
			},
			user: &okteto.User{
				Token: "test",
			},
			kubeconfigCtx: kubeconfigFields{
				Name:      []string{"cloud_okteto_com"},
				Namespace: []string{""},
			},
			expectedErr: false,
		},
		{
			name: "transform k8s to url and there is a context",
			ctxStore: &okteto.OktetoContextStore{
				Contexts: map[string]*okteto.OktetoContext{
					"https://cloud.okteto.com": {
						Token:    "this is a token",
						IsOkteto: true,
					},
				},
			},
			user: &okteto.User{
				Token: "test",
			},
			ctxOptions: &ContextOptions{
				isOkteto: false,
				Context:  "cloud_okteto_com",
			},
			kubeconfigCtx: kubeconfigFields{[]string{"cloud_okteto_com"}, []string{"test"}, ""},
			expectedErr:   false,
		},
		{
			name: "change to available okteto context",
			ctxStore: &okteto.OktetoContextStore{
				Contexts: map[string]*okteto.OktetoContext{
					"https://cloud.okteto.com": {
						Token:    "this is a token",
						IsOkteto: true,
					},
				},
			},
			user: &okteto.User{
				Token: "test",
			},
			ctxOptions: &ContextOptions{
				isOkteto: true,
				Context:  "cloud.okteto.com",
			},
			kubeconfigCtx: kubeconfigFields{[]string{"cloud_okteto_com"}, []string{"test"}, ""},
			expectedErr:   false,
		},
		{
			name: "change to available okteto context",
			ctxStore: &okteto.OktetoContextStore{
				Contexts: map[string]*okteto.OktetoContext{
					"https://cloud.okteto.com": {
						Token:    "this is a token",
						IsOkteto: true,
					},
				},
			},
			user: &okteto.User{
				Token: "test",
			},
			ctxOptions: &ContextOptions{
				isOkteto: true,
				Context:  "https://cloud.okteto.com",
			},
			kubeconfigCtx: kubeconfigFields{[]string{"cloud_okteto_com"}, []string{"test"}, ""},
			expectedErr:   false,
		},

		{
			name: "empty ctx create url",
			ctxStore: &okteto.OktetoContextStore{
				Contexts: make(map[string]*okteto.OktetoContext),
			},
			ctxOptions: &ContextOptions{
				isOkteto: true,
				Context:  "https://okteto.cloud.com",
				Token:    "this is a token",
			},
			user: &okteto.User{
				Token: "test",
			},
			kubeconfigCtx: kubeconfigFields{[]string{"cloud_okteto_com"}, []string{"test"}, ""},
			expectedErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := createKubeconfig(tt.kubeconfigCtx); err != nil {
				t.Fatal(err)
			}
			fakeK8sClient := fakeK8sProvider{objects: tt.fakeObjects}
			fakeUserClient := fakeUserClient{userContext: &okteto.UserContext{User: *tt.user}}
			ctxController := ContextUseController{
				k8sClientProvider: fakeK8sClient.K8sProvider,
				loginController: login.FakeLoginController{
					User: tt.user,
				},
				oktetoUserClientProvider: fakeUserClient.userClientProvider,
			}
			okteto.CurrentStore = tt.ctxStore
			err := ctxController.UseContext(ctx, tt.ctxOptions)
			if err != nil && !tt.expectedErr {
				t.Fatalf("Not expecting error but got: %s", err.Error())
			} else if tt.expectedErr && err == nil {
				t.Fatal("Not thrown error")
			}
		})
	}
}
