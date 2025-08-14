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

	k8sdivert "github.com/okteto/okteto/pkg/divert/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/testing"
)

// Divert implements DivertInterface
type Divert struct {
	getErr, createErr, updateErr, deleteErr, listErr error
	Fake                                              *V1
	ns                                                string
}

var divertResource = schema.GroupVersionResource{Group: k8sdivert.GroupName, Version: k8sdivert.GroupVersion, Resource: k8sdivert.DivertResource}

var divertKind = schema.GroupVersionKind{Group: k8sdivert.GroupName, Version: k8sdivert.GroupVersion, Kind: k8sdivert.DivertKind}

func (c *Divert) Create(_ context.Context, divert *k8sdivert.Divert, _ metav1.CreateOptions) (*k8sdivert.Divert, error) {
	if c.createErr != nil {
		return nil, c.createErr
	}

	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(divertResource, c.ns, divert), &k8sdivert.Divert{})

	if obj == nil {
		return nil, err
	}
	return obj.(*k8sdivert.Divert), err
}

func (c *Divert) Update(_ context.Context, divert *k8sdivert.Divert) (*k8sdivert.Divert, error) {
	if c.updateErr != nil {
		return nil, c.updateErr
	}

	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(divertResource, c.ns, divert), &k8sdivert.Divert{})

	if obj == nil {
		return nil, err
	}
	return obj.(*k8sdivert.Divert), err
}

func (c *Divert) Get(_ context.Context, name string, _ metav1.GetOptions) (*k8sdivert.Divert, error) {
	if c.getErr != nil {
		return nil, c.getErr
	}

	obj, err := c.Fake.
		Invokes(testing.NewGetAction(divertResource, c.ns, name), &k8sdivert.Divert{})

	if obj == nil {
		return &k8sdivert.Divert{}, err
	}
	return obj.(*k8sdivert.Divert), err
}

func (c *Divert) Delete(_ context.Context, name string, options metav1.DeleteOptions) error {
	if c.deleteErr != nil {
		return c.deleteErr
	}

	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(divertResource, c.ns, name), &k8sdivert.Divert{})

	return err
}

func (c *Divert) List(ctx context.Context, opts metav1.ListOptions) (*k8sdivert.DivertList, error) {
	if c.listErr != nil {
		return nil, c.listErr
	}
	obj, err := c.Fake.
		Invokes(testing.NewListAction(divertResource, divertKind, c.ns, opts), &k8sdivert.DivertList{})

	if obj == nil {
		return nil, err
	}
	return obj.(*k8sdivert.DivertList), err
}