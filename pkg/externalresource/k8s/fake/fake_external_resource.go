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

package fake

import (
	"context"

	k8sexternalresource "github.com/okteto/okteto/pkg/externalresource/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/testing"
)

// ExternalResource implements ExternalResourceInterface
type ExternalResource struct {
	getErr, createErr, updateErr, listErr error
	Fake                                  *V1
	ns                                    string
}

var externalResourceResource = schema.GroupVersionResource{Group: k8sexternalresource.GroupName, Version: k8sexternalresource.GroupVersion, Resource: k8sexternalresource.ExternalResourceResource}

var externalResourceKind = schema.GroupVersionKind{Group: k8sexternalresource.GroupName, Version: k8sexternalresource.GroupVersion, Kind: k8sexternalresource.ExternalResourceKind}

func (c *ExternalResource) Create(_ context.Context, external *k8sexternalresource.External, _ metav1.CreateOptions) (*k8sexternalresource.External, error) {
	if c.createErr != nil {
		return nil, c.createErr
	}

	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(externalResourceResource, c.ns, external), &k8sexternalresource.External{})

	if obj == nil {
		return nil, err
	}
	return obj.(*k8sexternalresource.External), err
}

func (c *ExternalResource) Update(_ context.Context, external *k8sexternalresource.External) (*k8sexternalresource.External, error) {
	if c.updateErr != nil {
		return nil, c.updateErr
	}

	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(externalResourceResource, c.ns, external), &k8sexternalresource.External{})

	if obj == nil {
		return nil, err
	}
	return obj.(*k8sexternalresource.External), err
}

func (c *ExternalResource) Get(_ context.Context, name string, _ metav1.GetOptions) (*k8sexternalresource.External, error) {
	if c.getErr != nil {
		return nil, c.getErr
	}

	obj, err := c.Fake.
		Invokes(testing.NewGetAction(externalResourceResource, c.ns, name), &k8sexternalresource.External{})

	if obj == nil {
		return &k8sexternalresource.External{}, err
	}
	return obj.(*k8sexternalresource.External), err
}

func (c *ExternalResource) List(ctx context.Context, opts metav1.ListOptions) (*k8sexternalresource.ExternalList, error) {
	if c.listErr != nil {
		return nil, c.listErr
	}
	obj, err := c.Fake.
		Invokes(testing.NewListAction(externalResourceResource, externalResourceKind, c.ns, opts), &k8sexternalresource.ExternalList{})

	if obj == nil {
		return nil, err
	}
	return obj.(*k8sexternalresource.ExternalList), err
}
