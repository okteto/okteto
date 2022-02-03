// Copyright 2022 The Okteto Authors
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
	"regexp"
	"strings"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/types"
	"github.com/shurcooL/graphql"
)

const (
	// Maximum number of characters allowed in a namespace name
	MAX_ALLOWED_CHARS = 63
)

type namespaceClient struct {
	client *graphql.Client
}

func newNamespaceClient(client *graphql.Client) *namespaceClient {
	return &namespaceClient{client: client}
}

// CreateNamespace creates a namespace
func (c *namespaceClient) Create(ctx context.Context, namespace string) (string, error) {
	var mutation struct {
		Space struct {
			Id graphql.String
		} `graphql:"createSpace(name: $name)"`
	}
	variables := map[string]interface{}{
		"name": graphql.String(namespace),
	}
	err := mutate(ctx, &mutation, variables, c.client)
	if err != nil {
		return "", err
	}

	return string(mutation.Space.Id), nil
}

// List list namespaces
func (c *namespaceClient) List(ctx context.Context) ([]types.Namespace, error) {
	var queryStruct struct {
		Spaces []struct {
			Id     graphql.String
			Status graphql.String
		} `graphql:"spaces"`
	}

	err := query(ctx, &queryStruct, nil, c.client)
	if err != nil {
		if strings.Contains(err.Error(), "Cannot query field \"status\" on type \"Space\"") {
			return c.deprecatedListNamespaces(ctx)
		}
		return nil, err
	}

	result := make([]types.Namespace, 0)
	for _, space := range queryStruct.Spaces {
		result = append(result, types.Namespace{
			ID:     string(space.Id),
			Status: string(space.Status),
		})
	}

	return result, nil
}

// TODO: remove when all users are in OktetoEnterprise >= 10.6
func (c *namespaceClient) deprecatedListNamespaces(ctx context.Context) ([]types.Namespace, error) {
	var queryStruct struct {
		Spaces []struct {
			Id       graphql.String
			Sleeping graphql.Boolean
		} `graphql:"spaces"`
	}

	err := query(ctx, &queryStruct, nil, c.client)
	if err != nil {
		return nil, err
	}

	result := make([]types.Namespace, 0)
	for _, space := range queryStruct.Spaces {
		status := "Active"
		if space.Sleeping {
			status = "Sleeping"
		}
		result = append(result, types.Namespace{
			ID:     string(space.Id),
			Status: status,
		})
	}

	return result, nil
}

// AddNamespaceMembers adds members to a namespace
func (c *namespaceClient) AddMembers(ctx context.Context, namespace string, members []string) error {
	var mutation struct {
		Space struct {
			Id graphql.String
		} `graphql:"updateSpace(id: $id, members: $members)"`
	}

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
	var mutation struct {
		Space struct {
			Id graphql.String
		} `graphql:"deleteSpace(id: $id)"`
	}
	variables := map[string]interface{}{
		"id": graphql.String(namespace),
	}
	err := mutate(ctx, &mutation, variables, c.client)
	if err != nil {
		return err
	}

	return nil
}

func validateNamespace(namespace, object string) error {
	if len(namespace) > MAX_ALLOWED_CHARS {
		return oktetoErrors.UserError{
			E:    fmt.Errorf("invalid %s name", object),
			Hint: fmt.Sprintf("%s name must be shorter than 63 characters.", object),
		}
	}
	nameValidationRegex := regexp.MustCompile("^[a-z0-9]([-a-z0-9]*[a-z0-9])?$")
	if !nameValidationRegex.MatchString(namespace) {
		return oktetoErrors.UserError{
			E:    fmt.Errorf("invalid %s name", object),
			Hint: fmt.Sprintf("%s name must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character", object),
		}
	}
	return nil
}

// SleepNamespace sleeps a namespace
func (c *namespaceClient) SleepNamespace(ctx context.Context, namespace string) error {
	var mutation struct {
		Space struct {
			Id graphql.String
		} `graphql:"sleepSpace(space: $space)"`
	}
	variables := map[string]interface{}{
		"space": graphql.String(namespace),
	}
	err := mutate(ctx, &mutation, variables, c.client)
	if err != nil {
		return err
	}

	return nil
}
