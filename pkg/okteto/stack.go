// Copyright 2020 The Okteto Authors
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

	"github.com/okteto/okteto/pkg/model"
	"github.com/shurcooL/graphql"
)

func (c *OktetoClient) ListStackEndpoints(ctx context.Context, stack *model.Stack) ([]Endpoint, error) {
	var query struct {
		Space struct {
			Stacks []struct {
				Id   graphql.String
				Name graphql.String
			}
			Deployments []struct {
				DeployedBy graphql.String
				Endpoints  []struct {
					Url graphql.String
				}
			}
			Statefulsets []struct {
				DeployedBy graphql.String
				Endpoints  []struct {
					Url graphql.String
				}
			}
		} `graphql:"space(id: $id)"`
	}

	variables := map[string]interface{}{
		"id": graphql.String(stack.Namespace),
	}
	endpoints := make([]Endpoint, 0)

	err := c.client.Query(ctx, &query, variables)
	if err != nil {
		return nil, translateAPIErr(err)
	}

	var stackId string
	for _, queriedStack := range query.Space.Stacks {
		if stack.Name == string(queriedStack.Name) {
			stackId = string(queriedStack.Id)
		}
	}
	if stackId == "" {
		return nil, fmt.Errorf("stack '%s' not found", stack.Name)
	}

	for _, d := range query.Space.Deployments {
		if string(d.DeployedBy) != stackId {
			continue
		}
		for _, endpoint := range d.Endpoints {
			endpoints = append(endpoints, Endpoint{
				URL: string(endpoint.Url),
			})
		}
	}

	for _, sfs := range query.Space.Statefulsets {
		if string(sfs.DeployedBy) != stackId {
			continue
		}
		for _, endpoint := range sfs.Endpoints {
			endpoints = append(endpoints, Endpoint{
				URL: string(endpoint.Url),
			})
		}
	}
	return endpoints, nil
}
