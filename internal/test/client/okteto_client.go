// Copyright 2022 The Okteto Authors
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
	"github.com/okteto/okteto/pkg/types"
)

type FakeOktetoClientProvider struct {
	c   *FakeOktetoClient
	err error
}

func NewFakeOktetoClientProvider(f *FakeOktetoClient) *FakeOktetoClientProvider {
	return &FakeOktetoClientProvider{
		c: f,
	}
}

func (p *FakeOktetoClientProvider) Provide() (types.OktetoInterface, error) {
	return p.c, p.err
}

type FakeOktetoClient struct {
	Namespace types.NamespaceInterface
	Users     types.UserInterface
	Preview   types.PreviewInterface
}

func NewFakeOktetoClient() *FakeOktetoClient {
	return &FakeOktetoClient{}
}

// Namespace retrieves the NamespaceClient
func (c *FakeOktetoClient) Namespaces() types.NamespaceInterface {
	return c.Namespace
}

// Namespace retrieves the NamespaceClient
func (c *FakeOktetoClient) Previews() types.PreviewInterface {
	return c.Preview
}

// User retrieves the UserClient
func (c *FakeOktetoClient) User() types.UserInterface {
	return c.Users
}
