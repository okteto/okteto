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

package client

import (
	"context"
	"fmt"

	"github.com/okteto/okteto/pkg/types"
)

// FakeNamespaceClient mocks the namespace interface
type FakeNamespaceClient struct {
	err        error
	namespaces []types.Namespace

	// WakeCalls is the number of times Wake was called
	WakeCalls int
}

func NewFakeNamespaceClient(ns []types.Namespace, err error) *FakeNamespaceClient {
	return &FakeNamespaceClient{namespaces: ns, err: err}
}

// Create creates a namespace
func (c *FakeNamespaceClient) Create(_ context.Context, namespace string) (string, error) {
	c.namespaces = append(c.namespaces, types.Namespace{ID: namespace})
	return namespace, c.err
}

// List list namespaces
func (c *FakeNamespaceClient) List(_ context.Context) ([]types.Namespace, error) {
	return c.namespaces, c.err
}

// AddMembers adds members to a namespace
func (c *FakeNamespaceClient) AddMembers(_ context.Context, _ string, _ []string) error {
	return c.err
}

// Delete deletes a namespace
func (c *FakeNamespaceClient) Delete(_ context.Context, namespace string) error {
	var updatedNamespaces []types.Namespace
	for _, ns := range c.namespaces {
		if ns.ID != namespace {
			updatedNamespaces = append(updatedNamespaces, ns)
		}
	}
	// if updated are same as current, namespace was not found
	if len(updatedNamespaces) == len(c.namespaces) {
		return fmt.Errorf("not found")
	}
	// override with updated
	c.namespaces = updatedNamespaces
	return nil
}

// Sleep deletes a namespace
func (c *FakeNamespaceClient) Sleep(_ context.Context, _ string) error {
	return c.err
}

// DestroyAll deletes a namespace
func (*FakeNamespaceClient) DestroyAll(_ context.Context, _ string, _ bool) error {
	return nil
}

// Wake wakes up a namespace
func (c *FakeNamespaceClient) Wake(_ context.Context, _ string) error {
	if c.err != nil {
		return c.err
	}

	c.WakeCalls++
	return nil
}
