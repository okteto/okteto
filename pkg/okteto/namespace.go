// Copyright 2021 The Okteto Authors
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

	"github.com/okteto/okteto/pkg/errors"
	"github.com/shurcooL/graphql"
)

const (
	// Maximum number of characters allowed in a namespace name
	MAX_ALLOWED_CHARS = 63
)

//Namespace represents an Okteto k8s namespace
type Namespace struct {
	ID       string `json:"id" yaml:"id"`
	Sleeping bool   `json:"sleeping" yaml:"sleeping"`
}

// CreateNamespace creates a namespace
func (c *OktetoClient) CreateNamespace(ctx context.Context, namespace string) (string, error) {
	var mutation struct {
		Space struct {
			Id graphql.String
		} `graphql:"createSpace(name: $name)"`
	}
	variables := map[string]interface{}{
		"name": graphql.String(namespace),
	}
	err := c.Mutate(ctx, &mutation, variables)
	if err != nil {
		return "", err
	}

	return string(mutation.Space.Id), nil
}

// ListNamespaces list namespaces
func (c *OktetoClient) ListNamespaces(ctx context.Context) ([]Namespace, error) {
	var query struct {
		Spaces []struct {
			Id       graphql.String
			Sleeping graphql.Boolean
		} `graphql:"spaces"`
	}

	err := c.Query(ctx, &query, nil)
	if err != nil {
		return nil, err
	}

	result := make([]Namespace, 0)
	for _, space := range query.Spaces {
		result = append(result, Namespace{
			ID:       string(space.Id),
			Sleeping: bool(space.Sleeping),
		})
	}

	return result, nil
}

// AddNamespaceMembers adds members to a namespace
func (c *OktetoClient) AddNamespaceMembers(ctx context.Context, namespace string, members []string) error {
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
	err := c.Mutate(ctx, &mutation, variables)
	if err != nil {
		return err
	}

	return nil
}

// DeleteNamespace deletes a namespace
func (c *OktetoClient) DeleteNamespace(ctx context.Context, namespace string) error {
	var mutation struct {
		Space struct {
			Id graphql.String
		} `graphql:"deleteSpace(id: $id)"`
	}
	variables := map[string]interface{}{
		"id": graphql.String(namespace),
	}
	err := c.Mutate(ctx, &mutation, variables)
	if err != nil {
		return err
	}

	return nil
}

func validateNamespace(namespace, object string) error {
	if len(namespace) > MAX_ALLOWED_CHARS {
		return errors.UserError{
			E:    fmt.Errorf("invalid %s name", object),
			Hint: fmt.Sprintf("%s name must be shorter than 63 characters.", object),
		}
	}
	nameValidationRegex := regexp.MustCompile("^[a-z0-9]([-a-z0-9]*[a-z0-9])?$")
	if !nameValidationRegex.MatchString(namespace) {
		return errors.UserError{
			E:    fmt.Errorf("invalid %s name", object),
			Hint: fmt.Sprintf("%s name must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character", object),
		}
	}
	return nil
}
