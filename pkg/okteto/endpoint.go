package okteto

import (
	"context"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/shurcooL/graphql"
)

type endpointClient struct {
	client graphqlClientInterface
}

func newEndpointClient(client graphqlClientInterface) *endpointClient {
	return &endpointClient{
		client: client,
	}
}

type listEndpointsQuery struct {
	Response []string `graphql:"endpoints(space: $space, label: $label)"`
}

func (c *endpointClient) List(ctx context.Context, ns, label string) ([]string, error) {
	oktetoLog.Infof("listing endpoints in '%s' namespace with label %s", ns, label)
	var queryStruct listEndpointsQuery

	variables := map[string]interface{}{
		"space": graphql.String(ns),
		"label": graphql.String(label),
	}

	err := query(ctx, &queryStruct, variables, c.client)
	if err != nil {
		return nil, err
	}

	return queryStruct.Response, nil
}
