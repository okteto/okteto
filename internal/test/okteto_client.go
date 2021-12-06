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

package test

import (
	"context"

	"github.com/okteto/okteto/pkg/types"
)

type FakeOktetoClientProvider struct {
	UserContext *types.UserContext
	Err         error
	Namespaces  []types.Namespace
	Previews    []types.Preview
}

func NewFakeOktetoClientProvider(userContext *types.UserContext, namespaces []types.Namespace, err error) *FakeOktetoClientProvider {
	return &FakeOktetoClientProvider{UserContext: userContext, Namespaces: namespaces, Err: err}
}

func (f FakeOktetoClientProvider) NewOktetoUserClient() (types.UserInterface, error) {
	return FakeUserClient{UserContext: f.UserContext, err: f.Err}, nil
}

func (f FakeOktetoClientProvider) NewOktetoNamespaceClient() (types.NamespaceInterface, error) {
	return FakeNamespaceClient{namespaces: f.Namespaces}, nil
}

type FakeUserClient struct {
	UserContext *types.UserContext
	err         error
}

// GetUserContext get user context
func (f FakeUserClient) GetUserContext(ctx context.Context) (*types.UserContext, error) {
	return f.UserContext, f.err
}

type FakeNamespaceClient struct {
	namespaces []types.Namespace
	err        error
	previews   []types.Preview
}

func NewFakeNamespaceClient(namespaces []types.Namespace, previews []types.Preview, err error) *FakeNamespaceClient {
	return &FakeNamespaceClient{namespaces: namespaces, err: err, previews: previews}
}

// GetUserContext get user context
func (f FakeNamespaceClient) ListNamespaces(ctx context.Context) ([]types.Namespace, error) {
	return f.namespaces, f.err
}

// GetUserContext get user context
func (f FakeNamespaceClient) ListPreviews(ctx context.Context) ([]types.Preview, error) {
	return f.previews, f.err
}
