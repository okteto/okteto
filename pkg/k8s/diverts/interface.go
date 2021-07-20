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

package diverts

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
)

const divertsResource = "diverts"

type DivertInterface interface {
	List(ctx context.Context, opts metav1.ListOptions) (*DivertList, error)
	Get(ctx context.Context, name string, options metav1.GetOptions) (*Divert, error)
	Create(ctx context.Context, divert *Divert) (*Divert, error)
	Update(ctx context.Context, divert *Divert) (*Divert, error)
	Delete(ctx context.Context, name string, options metav1.DeleteOptions) error
}

type divertClient struct {
	restClient rest.Interface
	scheme     *runtime.Scheme
	ns         string
}

func (c *divertClient) List(ctx context.Context, opts metav1.ListOptions) (*DivertList, error) {
	result := DivertList{}
	err := c.restClient.
		Get().
		Namespace(c.ns).
		Resource(divertsResource).
		VersionedParams(&opts, runtime.NewParameterCodec(c.scheme)).
		Do(ctx).
		Into(&result)
	return &result, err
}

func (c *divertClient) Get(ctx context.Context, name string, opts metav1.GetOptions) (*Divert, error) {
	result := Divert{}
	err := c.restClient.
		Get().
		Namespace(c.ns).
		Resource(divertsResource).
		Name(name).
		VersionedParams(&opts, runtime.NewParameterCodec(c.scheme)).
		Do(ctx).
		Into(&result)
	return &result, err
}

func (c *divertClient) Create(ctx context.Context, divert *Divert) (*Divert, error) {
	result := Divert{}
	err := c.restClient.
		Post().
		Namespace(c.ns).
		Resource(divertsResource).
		Body(divert).
		Do(ctx).
		Into(&result)

	return &result, err
}

func (c *divertClient) Update(ctx context.Context, divert *Divert) (*Divert, error) {
	result := Divert{}
	err := c.restClient.
		Put().
		Namespace(c.ns).
		Resource(divertsResource).
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
		Resource(divertsResource).
		Name(name).
		Do(ctx).Error()
}
