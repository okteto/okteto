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

package k8s

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sScheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

const (
	// GroupName k8s group name for the divert resource
	GroupName = "okteto.com"
	// GroupVersion k8s version for Divert resource
	GroupVersion = "v1"
)

var (
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
)

var schemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: GroupVersion}

// DivertV1Interface defines a method to get a DivertInterface
type DivertV1Interface interface {
	Diverts(namespace string) DivertInterface
}

// DivertV1Client client to work with Diverts v1 resources
type DivertV1Client struct {
	restClient rest.Interface
	scheme     *runtime.Scheme
}

// GetDivertClient returns a new DivertV1Interface client
func GetDivertClient(config *rest.Config) (DivertV1Interface, error) {
	c, err := NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// NewForConfig creates a new client for DivertV1 or error
func NewForConfig(cfg *rest.Config) (*DivertV1Client, error) {
	scheme := runtime.NewScheme()
	if err := SchemeBuilder.AddToScheme(scheme); err != nil {
		return nil, err
	}
	config := *cfg
	config.GroupVersion = &schemeGroupVersion
	config.APIPath = "/apis"
	config.ContentType = runtime.ContentTypeJSON
	config.NegotiatedSerializer = k8sScheme.Codecs.WithoutConversion()

	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &DivertV1Client{restClient: client, scheme: scheme}, nil
}

// Diverts returns an instance of DivertInterface
func (c *DivertV1Client) Diverts(namespace string) DivertInterface {
	return &divertClient{
		restClient: c.restClient,
		scheme:     c.scheme,
		ns:         namespace,
	}
}

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(schemeGroupVersion,
		&Divert{},
		&DivertList{},
	)

	metav1.AddToGroupVersion(scheme, schemeGroupVersion)
	return nil
}