package client

import (
	"context"
)

// FakeSSEClient mocks the previews interface
type FakeSSEClient struct {
	response *FakeSSEResponse
}

type FakeSSEResponse struct {
	StreamErr error
}

// NewFakeSSEClient returns a new fake preview client
func NewFakeSSEClient(response *FakeSSEResponse) *FakeSSEClient {
	return &FakeSSEClient{
		response: response,
	}
}

// List list namespaces
func (c *FakeSSEClient) StreamPipelineLogs(ctx context.Context, _, _, _ string) error {
	return c.response.StreamErr
}
