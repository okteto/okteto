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
	DeployedBy graphql.String
	Endpoints  []EndpointInfo
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
	for _, endpoint := range filterEndpointsFromComponent(queryStruct.Response.Externals, deployedBy) {
		endpoints = append(endpoints, fmt.Sprintf("%s (external)", endpoint))
	}
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
