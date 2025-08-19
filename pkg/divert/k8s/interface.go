// Copyright 2025 The Okteto Authors
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
	// DivertResource defines the resource of Divert
	DivertResource = "diverts"

	// DivertKind defines the kind of the resource divert
	DivertKind = "Divert"
)

type divertClient struct {
	restClient rest.Interface
	scheme     *runtime.Scheme
	ns         string
}

// DivertInterface defines the operations for the divert resource item Kubernetes client
type DivertInterface interface {
	Create(ctx context.Context, divert *Divert, options metav1.CreateOptions) (*Divert, error)
	Get(ctx context.Context, name string, options metav1.GetOptions) (*Divert, error)
	Update(ctx context.Context, divert *Divert) (*Divert, error)
	Delete(ctx context.Context, name string, options metav1.DeleteOptions) error
	List(ctx context.Context, options metav1.ListOptions) (*DivertList, error)
}

func (c *divertClient) Create(ctx context.Context, divert *Divert, _ metav1.CreateOptions) (*Divert, error) {
	result := Divert{}
	err := c.restClient.
		Post().
		Namespace(c.ns).
		Resource(DivertResource).
		Body(divert).
		Do(ctx).
		Into(&result)

	return &result, err
}

func (c *divertClient) Get(ctx context.Context, name string, opts metav1.GetOptions) (*Divert, error) {
	result := Divert{}
	err := c.restClient.
		Get().
		Namespace(c.ns).
		Resource(DivertResource).
		Name(name).
		VersionedParams(&opts, runtime.NewParameterCodec(c.scheme)).
		Do(ctx).
		Into(&result)
	return &result, err
}

func (c *divertClient) Update(ctx context.Context, divert *Divert) (*Divert, error) {
	result := Divert{}
	err := c.restClient.
		Put().
		Namespace(c.ns).
		Resource(DivertResource).
		Name(divert.Name).
		Body(divert).
		Do(ctx).
		Into(&result)

	return &result, err
}

func (c *divertClient) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return c.restClient.
		Delete().
		Namespace(c.ns).
		Resource(DivertResource).
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

func (c *divertClient) List(ctx context.Context, opts metav1.ListOptions) (*DivertList, error) {
	result := DivertList{}
	err := c.restClient.Get().
		Namespace(c.ns).
		Resource(DivertResource).
		VersionedParams(&opts, runtime.NewParameterCodec(c.scheme)).
		Do(ctx).
		Into(&result)
	return &result, err
}
