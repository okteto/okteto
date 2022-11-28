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
func (c *FakeSSEClient) StreamPipelineLogs(ctx context.Context, _, _, _ string) error {
	return c.response.StreamErr
}
