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

package client

import (
	"context"

	"github.com/okteto/okteto/pkg/types"
)

// FakeCatalogResponse holds the canned responses returned by FakeCatalogClient.
type FakeCatalogResponse struct {
	ErrList   error
	ErrDeploy error

	CatalogItems []types.GitCatalogItem
	DeployResp   *types.GitDeployResponse

	// LastDeployOpts captures the opts passed to the last Deploy call so tests can assert on them.
	LastDeployOpts types.CatalogDeployOptions
}

// FakeCatalogClient mocks the CatalogInterface for tests.
type FakeCatalogClient struct {
	response *FakeCatalogResponse
}

// NewFakeCatalogClient returns a FakeCatalogClient backed by the given response.
func NewFakeCatalogClient(response *FakeCatalogResponse) *FakeCatalogClient {
	if response == nil {
		response = &FakeCatalogResponse{}
	}
	return &FakeCatalogClient{response: response}
}

// List returns the configured catalog items.
func (c *FakeCatalogClient) List(_ context.Context) ([]types.GitCatalogItem, error) {
	return c.response.CatalogItems, c.response.ErrList
}

// Deploy records the opts and returns the canned deploy response.
func (c *FakeCatalogClient) Deploy(_ context.Context, opts types.CatalogDeployOptions) (*types.GitDeployResponse, error) {
	c.response.LastDeployOpts = opts
	return c.response.DeployResp, c.response.ErrDeploy
}
