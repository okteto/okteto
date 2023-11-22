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
	endpoints = append(endpoints, filterEndpointsFromComponent(queryStruct.Response.Deployments, deployedBy)...)
	endpoints = append(endpoints, filterEndpointsFromComponent(queryStruct.Response.Statefulsets, deployedBy)...)
	endpoints = append(endpoints, filterEndpointsFromComponent(queryStruct.Response.Externals, deployedBy)...)
	endpoints = append(endpoints, filterEndpointsFromComponent(queryStruct.Response.Functions, deployedBy)...)

	return endpoints, nil
}

func filterEndpointsFromComponent(components []Component, deployedBy string) []string {
	var endpoints []string
	for _, component := range components {
		if component.DeployedBy == graphql.String(deployedBy) {
			for _, endpoint := range component.Endpoints {
				endpoints = append(endpoints, string(endpoint.Url))
			}
		}
	}

	return endpoints
}
