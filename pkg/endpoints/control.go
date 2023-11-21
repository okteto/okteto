package endpoints

import (
	"context"

	"github.com/okteto/okteto/pkg/okteto"
)

type APIControl struct {
	OktetoClient *okteto.OktetoClient
}

func NewEndpointControl() (*APIControl, error) {
	c, err := okteto.NewOktetoClient()
	if err != nil {
		return nil, err
	}
	return &APIControl{
		OktetoClient: c,
	}, nil
}

func (c *APIControl) List(ctx context.Context, ns string, labelSelector string) ([]string, error) {
	return c.OktetoClient.Endpoint().List(ctx, ns, labelSelector)
}
