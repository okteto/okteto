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
	"errors"
	"fmt"
	"strings"

	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/types"
	"github.com/shurcooL/graphql"
)

// ErrCatalogNotSupported is returned when the Okteto backend does not expose
// the catalog feature (older servers).
var ErrCatalogNotSupported = oktetoErrors.UserError{
	E:    errors.New("the catalog feature is not available on this Okteto instance"),
	Hint: "Please upgrade Okteto or contact your system administrator for more information.",
}

const catalogItemIDArgUnknown = `Unknown argument "catalogItemId" on field "deployGitRepository"`

type catalogClient struct {
	client graphqlClientInterface
}

func newCatalogClient(client graphqlClientInterface) *catalogClient {
	return &catalogClient{client: client}
}

type getGitCatalogItemsQuery struct {
	Response []gitCatalogItem `graphql:"getGitCatalogItems"`
}

type gitCatalogItem struct {
	Id            graphql.String
	Name          graphql.String
	RepositoryUrl graphql.String
	Branch        graphql.String
	ManifestPath  graphql.String
	ReadOnly      graphql.Boolean
	Variables     []gitCatalogItemVariable
}

type gitCatalogItemVariable struct {
	Name  graphql.String
	Value graphql.String
}

type deployCatalogItemMutation struct {
	Response deployCatalogItemResponse `graphql:"deployGitRepository(name: $name, repository: $repository, space: $space, branch: $branch, variables: $variables, filename: $filename, catalogItemId: $catalogItemId, dependencies: $dependencies, source: $source)"`
}

type deployCatalogItemResponse struct {
	Action    actionStruct
	GitDeploy gitDeployInfoWithRepoInfo
}

// List returns the catalog items visible to the current user.
func (c *catalogClient) List(ctx context.Context) ([]types.GitCatalogItem, error) {
	oktetoLog.Infof("listing catalog items")
	var q getGitCatalogItemsQuery
	if err := query(ctx, &q, nil, c.client); err != nil {
		if isCatalogUnsupportedErr(err) {
			return nil, ErrCatalogNotSupported
		}
		return nil, fmt.Errorf("failed to list catalog items: %w", err)
	}

	items := make([]types.GitCatalogItem, 0, len(q.Response))
	for _, it := range q.Response {
		vars := make([]types.GitCatalogItemVariable, 0, len(it.Variables))
		for _, v := range it.Variables {
			vars = append(vars, types.GitCatalogItemVariable{
				Name:  string(v.Name),
				Value: string(v.Value),
			})
		}
		items = append(items, types.GitCatalogItem{
			ID:            string(it.Id),
			Name:          string(it.Name),
			RepositoryURL: string(it.RepositoryUrl),
			Branch:        string(it.Branch),
			ManifestPath:  string(it.ManifestPath),
			ReadOnly:      bool(it.ReadOnly),
			Variables:     vars,
		})
	}
	return items, nil
}

// Deploy launches a catalog item into the given namespace.
// The deploy links back to the originating catalog item via catalogItemId so
// the UI can show a back-reference on the resulting dev environment.
func (c *catalogClient) Deploy(ctx context.Context, opts types.CatalogDeployOptions) (*types.GitDeployResponse, error) {
	oktetoLog.Infof("deploying catalog item '%s' into namespace '%s'", opts.Name, opts.Namespace)

	vars := c.buildDeployVariables(opts)
	mutation := &deployCatalogItemMutation{}
	if err := mutate(ctx, mutation, vars, c.client); err != nil {
		if strings.Contains(err.Error(), catalogItemIDArgUnknown) {
			return nil, ErrCatalogNotSupported
		}
		return nil, fmt.Errorf("failed to deploy catalog item: %w", err)
	}

	return &types.GitDeployResponse{
		Action: &types.Action{
			ID:     string(mutation.Response.Action.Id),
			Name:   string(mutation.Response.Action.Name),
			Status: string(mutation.Response.Action.Status),
		},
		GitDeploy: &types.GitDeploy{
			ID:         string(mutation.Response.GitDeploy.Id),
			Name:       string(mutation.Response.GitDeploy.Name),
			Repository: string(mutation.Response.GitDeploy.Repository),
			Status:     string(mutation.Response.GitDeploy.Status),
		},
	}, nil
}

func (c *catalogClient) buildDeployVariables(opts types.CatalogDeployOptions) map[string]interface{} {
	inputVars := make([]InputVariable, 0, len(opts.Variables)+1)
	hasOrigin := false
	for _, v := range opts.Variables {
		inputVars = append(inputVars, InputVariable{
			Name:  graphql.String(v.Name),
			Value: graphql.String(v.Value),
		})
		if v.Name == "OKTETO_ORIGIN" {
			hasOrigin = true
		}
	}
	if !hasOrigin {
		inputVars = append(inputVars, InputVariable{
			Name:  graphql.String("OKTETO_ORIGIN"),
			Value: graphql.String(config.GetDeployOrigin()),
		})
	}

	return map[string]interface{}{
		"name":          graphql.String(opts.Name),
		"repository":    graphql.String(opts.Repository),
		"space":         graphql.String(opts.Namespace),
		"branch":        graphql.String(opts.Branch),
		"variables":     inputVars,
		"filename":      graphql.String(opts.Filename),
		"catalogItemId": graphql.String(opts.CatalogItemID),
		"dependencies":  graphql.Boolean(opts.RedeployDependencies),
		"source":        graphql.String("cli"),
	}
}

// isCatalogUnsupportedErr returns true when the server does not know about the
// getGitCatalogItems query, which means the catalog feature is not available
// on this Okteto instance. We anchor on the exact prefix emitted by the GraphQL
// library so we don't misclassify unrelated server errors that happen to echo
// the query name.
func isCatalogUnsupportedErr(err error) bool {
	return strings.Contains(err.Error(), `Cannot query field "getGitCatalogItems"`)
}
