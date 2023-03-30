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

	"github.com/okteto/okteto/pkg/types"
	"github.com/shurcooL/graphql"
)

const (
	RunningStatus     = "running"
	NotRunningStatus  = "not-running"
	CompletedStatus   = "completed"
	PullingStatus     = "pulling"
	ProgressingStatus = "progressing"
	BootingStatus     = "booting"
	ErrorStatus       = "error"
)

var TransitionStatus = map[string]bool{
	BootingStatus:     true,
	ProgressingStatus: true,
	PullingStatus:     true,
}

type namespaceClient struct {
	client graphqlClientInterface
}

type createNamespaceMutation struct {
	Response namespaceID `graphql:"createSpace(name: $name)"`
}
type deleteNamespaceMutation struct {
	Response namespaceID `graphql:"deleteSpace(id: $id)"`
}

type addMembersMutation struct {
	Response namespaceID `graphql:"updateSpace(id: $id, members: $members)"`
}

type listNamespacesQuery struct {
	Response []namespaceStatus `graphql:"spaces"`
}

type wakeNamespaceMutation struct {
	Response namespaceID `graphql:"wakeSpace(space: $space)"`
}

type namespaceDestroyAllMutation struct {
	Response namespaceID `graphql:"destroyAllInSpace(id: $id, includeVolumes: $includeVolumes)"`
}

type sleepNamespaceMutation struct {
	Response namespaceID `graphql:"sleepSpace(space: $space)"`
}

type namespaceStatus struct {
	Id     graphql.String
	Status graphql.String
}

type namespaceID struct {
	Id graphql.String
}

func newNamespaceClient(client graphqlClientInterface) *namespaceClient {
	return &namespaceClient{client: client}
}

// CreateNamespace creates a namespace
func (c *namespaceClient) Create(ctx context.Context, namespace string) (string, error) {
	var mutation createNamespaceMutation
	variables := map[string]interface{}{
		"name": graphql.String(namespace),
	}
	err := mutate(ctx, &mutation, variables, c.client)
	if err != nil {
		return "", err
	}

	return string(mutation.Response.Id), nil
}

// List list namespaces
func (c *namespaceClient) List(ctx context.Context) ([]types.Namespace, error) {
	var queryStruct listNamespacesQuery

	err := query(ctx, &queryStruct, nil, c.client)
	if err != nil {
		return nil, err
	}

	result := make([]types.Namespace, 0)
	for _, space := range queryStruct.Response {
		result = append(result, types.Namespace{
			ID:     string(space.Id),
			Status: string(space.Status),
		})
	}

	return result, nil
}

// AddMembers adds members to a namespace
func (c *namespaceClient) AddMembers(ctx context.Context, namespace string, members []string) error {
	var mutation addMembersMutation

	membersVariable := make([]graphql.String, 0)
	for _, m := range members {
		membersVariable = append(membersVariable, graphql.String(m))
	}
	variables := map[string]interface{}{
		"id":      graphql.String(namespace),
		"members": membersVariable,
	}
	err := mutate(ctx, &mutation, variables, c.client)
	if err != nil {
		return err
	}

	return nil
}

// DeleteNamespace deletes a namespace
func (c *namespaceClient) Delete(ctx context.Context, namespace string) error {
	var mutation deleteNamespaceMutation
	variables := map[string]interface{}{
		"id": graphql.String(namespace),
	}
	err := mutate(ctx, &mutation, variables, c.client)
	if err != nil {
		return err
	}

	return nil
}

// Sleep sleeps a namespace
func (c *namespaceClient) Sleep(ctx context.Context, namespace string) error {
	var mutation sleepNamespaceMutation
	variables := map[string]interface{}{
		"space": graphql.String(namespace),
	}
	err := mutate(ctx, &mutation, variables, c.client)
	if err != nil {
		return err
	}

	return nil
}

// DestroyAll deletes a namespace
func (c *namespaceClient) DestroyAll(ctx context.Context, namespace string, destroyVolumes bool) error {
	var mutation namespaceDestroyAllMutation
	// includingVolumes so everything is cleaned up by default with this cmd
	variables := map[string]interface{}{
		"id":             graphql.String(namespace),
		"includeVolumes": graphql.Boolean(destroyVolumes),
	}
	err := mutate(ctx, &mutation, variables, c.client)
	if err != nil {
		return err
	}

	return nil
}

// Wake wakes a namespace
func (c *namespaceClient) Wake(ctx context.Context, namespace string) error {
	var mutation wakeNamespaceMutation
	variables := map[string]interface{}{
		"space": graphql.String(namespace),
	}
	err := mutate(ctx, &mutation, variables, c.client)
	if err != nil {
		return err
	}

	return nil
}
