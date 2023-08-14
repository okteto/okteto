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

import "github.com/okteto/okteto/pkg/types"

// FakeKubetokenClient mocks the kubetoken client interface
type FakeKubetokenClient struct {
	response FakeKubetokenResponse
}

// FakeKubetokenResponse mocks the kubetoken response
type FakeKubetokenResponse struct {
	Token types.KubeTokenResponse
	Err   error
}

// NewFakeKubetokenClient returns a new fake kubetoken client
func NewFakeKubetokenClient(response FakeKubetokenResponse) *FakeKubetokenClient {
	return &FakeKubetokenClient{
		response: response,
	}
}

// GetKubeToken returns a temp token
func (c *FakeKubetokenClient) GetKubeToken(_, _ string) (types.KubeTokenResponse, error) {
	return c.response.Token, c.response.Err
}

// CheckService returns a temp token
func (c *FakeKubetokenClient) CheckService(_, _ string) error {
	return c.response.Err
}
