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
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
)

const (
	// ExternalResourceResource defines the resource of ExternalResource
	ExternalResourceResource = "externals"

	// ExternalResourceKind defines the kind of the resource external
	ExternalResourceKind = "External"
)

type externalClient struct {
	restClient rest.Interface
	scheme     *runtime.Scheme
	ns         string
}

// ExternalResourceInterface defines the operations for the external resource item Kubernetes client
type ExternalResourceInterface interface {
	Update(ctx context.Context, external *External) (*External, error)
	Get(ctx context.Context, name string, options metav1.GetOptions) (*External, error)
	Create(ctx context.Context, external *External, options metav1.CreateOptions) (*External, error)
	List(ctx context.Context, options metav1.ListOptions) (*ExternalList, error)
}

func (c *externalClient) Create(ctx context.Context, external *External, _ metav1.CreateOptions) (*External, error) {
	result := External{}
	err := c.restClient.
		Post().
		Namespace(c.ns).
		Resource(ExternalResourceResource).
		Body(external).
		Do(ctx).
		Into(&result)

	return &result, err
}

func (c *externalClient) Get(ctx context.Context, name string, opts metav1.GetOptions) (*External, error) {
	result := External{}
	err := c.restClient.
		Get().
		Namespace(c.ns).
		Resource(ExternalResourceResource).
		Name(name).
		VersionedParams(&opts, runtime.NewParameterCodec(c.scheme)).
		Do(ctx).
		Into(&result)
	return &result, err
}

func (c *externalClient) Update(ctx context.Context, external *External) (*External, error) {
	result := External{}
	err := c.restClient.
		Put().
		Namespace(c.ns).
		Resource(ExternalResourceResource).
		Name(external.Name).
		Body(external).
		Do(ctx).
		Into(&result)

	return &result, err
}

func (c *externalClient) List(ctx context.Context, opts metav1.ListOptions) (*ExternalList, error) {
	result := ExternalList{}
	err := c.restClient.Get().
		Namespace(c.ns).
		Resource(ExternalResourceResource).
		VersionedParams(&opts, runtime.NewParameterCodec(c.scheme)).
		Do(ctx).
		Into(&result)
	return &result, err
}
