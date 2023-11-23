package endpoints

import (
	"context"

	"github.com/okteto/okteto/pkg/okteto"
)

type APIControl struct {
	OktetoClient *okteto.OktetoClient
}

func NewEndpointControl(c *okteto.OktetoClient) *APIControl {
	return &APIControl{
		OktetoClient: c,
	}
}

func (c *APIControl) List(ctx context.Context, ns string, labelSelector string) ([]string, error) {
	return c.OktetoClient.Endpoint().List(ctx, ns, labelSelector)
}
