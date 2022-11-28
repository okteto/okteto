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
	"context"
)

// FakeSSEClient mocks the sse client interface
type FakeSSEClient struct {
	response *FakeSSEResponse
}

// FakeSSEResponse mocks the sse response
type FakeSSEResponse struct {
	StreamErr error
}

// NewFakeSSEClient returns a new fake sse client
func NewFakeSSEClient(response *FakeSSEResponse) *FakeSSEClient {
	return &FakeSSEClient{
		response: response,
	}
}

// StreamPipelineLogs starts the streaming of pipeline logs
func (c *FakeSSEClient) StreamPipelineLogs(_ context.Context, _, _, _ string) error {
	return c.response.StreamErr
}
