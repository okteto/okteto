package okteto

import (
	"context"
	"fmt"

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

type EndpointInfo struct {
	Url graphql.String
}

type Component struct {
	Endpoints  []EndpointInfo
	DeployedBy graphql.String
}

type Space struct {
	Deployments  []Component
	Statefulsets []Component
	Functions    []Component
	Externals    []Component
}

type SpaceQuery struct {
	Response Space `graphql:"space(id: $id)"`
}

func (c *endpointClient) List(ctx context.Context, ns, deployedBy string) ([]string, error) {
	oktetoLog.Infof("listing endpoints in '%s' namespace deployed by %s", ns, deployedBy)
	var queryStruct SpaceQuery
	variables := map[string]interface{}{
		"id": graphql.String(ns),
	}

	err := query(ctx, &queryStruct, variables, c.client)
	if err != nil {
		return nil, err
	}

	endpoints := make([]string, 0)
	for _, deployment := range queryStruct.Response.Deployments {
		if deployment.DeployedBy == graphql.String(deployedBy) {
			for _, endpoint := range deployment.Endpoints {
				endpoints = append(endpoints, string(endpoint.Url))
			}
		}
	}

	for _, sfs := range queryStruct.Response.Statefulsets {
		if sfs.DeployedBy == graphql.String(deployedBy) {
			for _, endpoint := range sfs.Endpoints {
				endpoints = append(endpoints, string(endpoint.Url))
			}
		}
	}

	for _, external := range queryStruct.Response.Externals {
		if external.DeployedBy == graphql.String(deployedBy) {
			for _, endpoint := range external.Endpoints {
				endpoints = append(endpoints, fmt.Sprintf("%s (external)", endpoint.Url))
			}
		}
	}

	for _, function := range queryStruct.Response.Functions {
		if function.DeployedBy == graphql.String(deployedBy) {
			for _, endpoint := range function.Endpoints {
				endpoints = append(endpoints, string(endpoint.Url))
			}
		}
	}

	return endpoints, nil
}
