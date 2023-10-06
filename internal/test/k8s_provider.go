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

package test

import (
	"github.com/okteto/okteto/pkg/k8s/ingresses"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type FakeK8sProvider struct {
	objects             []runtime.Object
	client              *fake.Clientset
	restConfig          *rest.Config
	errGetIngressClient error
	ErrProvide          error
}

func NewFakeK8sProvider(objects ...runtime.Object) *FakeK8sProvider {
	if len(objects) > 0 {
		return &FakeK8sProvider{objects: objects}
	}
	return &FakeK8sProvider{}
}

func (f *FakeK8sProvider) Provide(_ *clientcmdapi.Config) (kubernetes.Interface, *rest.Config, error) {
	if f.ErrProvide != nil {
		return nil, nil, f.ErrProvide
	}
	if f.client != nil {
		return f.client, nil, nil
	}
	var c *fake.Clientset
	if len(f.objects) > 0 {
		c = fake.NewSimpleClientset(f.objects...)
	} else {
		c = fake.NewSimpleClientset()
	}

	f.client = c
	return c, f.restConfig, nil
}

func (f *FakeK8sProvider) GetIngressClient() (*ingresses.Client, error) {
	if f.errGetIngressClient != nil {
		return nil, f.errGetIngressClient
	}

	c := fake.NewSimpleClientset(f.objects...)
	return ingresses.NewIngressClient(c, true), nil
}
