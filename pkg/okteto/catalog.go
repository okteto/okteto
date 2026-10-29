// Copyright 2026 The Okteto Authors
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

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/shurcooL/graphql"
)

type getCatalogItemSecretVariablesQuery struct {
	Response catalogItemSecretVariables `graphql:"getGitCatalogItem(id: $id)"`
}

type catalogItemSecretVariables struct {
	SecretVariables []catalogItemSecretVariableName
}

type catalogItemSecretVariableName struct {
	Name graphql.String
}

// GetCatalogItemSecretVariables returns the names of the secret variables of the catalog item with the id provided.
// The API never returns their values. It returns ErrCatalogItemNotFound when there is no item with that id
func (c *pipelineClient) GetCatalogItemSecretVariables(ctx context.Context, id string) ([]string, error) {
	oktetoLog.Infof("getting the secret variables of catalog item '%s'", id)
	var queryStruct getCatalogItemSecretVariablesQuery
	variables := map[string]interface{}{
		"id": graphql.String(id),
	}
	if err := query(ctx, &queryStruct, variables, c.client); err != nil {
		return nil, err
	}

	names := make([]string, 0, len(queryStruct.Response.SecretVariables))
	for _, v := range queryStruct.Response.SecretVariables {
		names = append(names, string(v.Name))
	}
	return names, nil
}
