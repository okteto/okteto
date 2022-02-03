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
	"strings"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/types"
	"github.com/shurcooL/graphql"
)

type previewClient struct {
	client *graphql.Client
}

func newPreviewClient(client *graphql.Client) *previewClient {
	return &previewClient{client: client}
}

//PreviewEnv represents an Okteto preview environment
type PreviewEnv struct {
	ID       string `json:"id" yaml:"id"`
	Job      string `json:"job" yaml:"job"`
	Sleeping bool   `json:"sleeping" yaml:"sleeping"`
	Scope    string `json:"scope" yaml:"scope"`
}

type InputVariable struct {
	Name  graphql.String `json:"name" yaml:"name"`
	Value graphql.String `json:"value" yaml:"value"`
}

type PreviewScope graphql.String

// DeployPreview creates a preview environment
func (c *OktetoClient) DeployPreview(ctx context.Context, name, scope, repository, branch, sourceUrl, filename string, variables []types.Variable) (*types.PreviewResponse, error) {
	if err := validateNamespace(name, "preview environment"); err != nil {
		return nil, err
	}

	previewResponse := &types.PreviewResponse{}

	if len(variables) > 0 {
		var mutation struct {
			Preview struct {
				Action struct {
					Id     graphql.String
					Name   graphql.String
					Status graphql.String
				}
				Preview struct {
					Id graphql.String
				}
			} `graphql:"deployPreview(name: $name, scope: $scope, repository: $repository, branch: $branch, sourceUrl: $sourceURL, variables: $variables, filename: $filename)"`
		}

		variablesVariable := make([]InputVariable, 0)
		for _, v := range variables {
			variablesVariable = append(variablesVariable, InputVariable{
				Name:  graphql.String(v.Name),
				Value: graphql.String(v.Value),
			})
		}
		queryVariables := map[string]interface{}{
			"name":       graphql.String(name),
			"scope":      PreviewScope(scope),
			"repository": graphql.String(repository),
			"branch":     graphql.String(branch),
			"sourceURL":  graphql.String(sourceUrl),
			"variables":  variablesVariable,
			"filename":   graphql.String(filename),
		}
		err := mutate(ctx, &mutation, queryVariables, c.client)
		if err != nil {
			if strings.Contains(err.Error(), "Cannot query field \"action\" on type \"Preview\"") {
				return c.deprecatedDeployPreview(ctx, name, scope, repository, branch, sourceUrl, filename, variables)
			}
			return nil, translatePreviewAPIErr(err, name)
		}
		previewResponse.Action = &types.Action{
			ID:     string(mutation.Preview.Action.Id),
			Name:   string(mutation.Preview.Action.Name),
			Status: string(mutation.Preview.Action.Status),
		}
		previewResponse.Preview = &types.Preview{
			ID: string(mutation.Preview.Preview.Id),
		}
	} else {
		var mutation struct {
			Preview struct {
				Action struct {
					Id     graphql.String
					Name   graphql.String
					Status graphql.String
				}
				Preview struct {
					Id graphql.String
				}
			} `graphql:"deployPreview(name: $name, scope: $scope, repository: $repository, branch: $branch, sourceUrl: $sourceURL, filename: $filename)"`
		}

		queryVariables := map[string]interface{}{
			"name":       graphql.String(name),
			"scope":      PreviewScope(scope),
			"repository": graphql.String(repository),
			"branch":     graphql.String(branch),
			"sourceURL":  graphql.String(sourceUrl),
			"filename":   graphql.String(filename),
		}
		err := mutate(ctx, &mutation, queryVariables, c.client)
		if err != nil {
			if strings.Contains(err.Error(), "Cannot query field \"job\" on type \"Preview\"") {
				return c.deprecatedDeployPreview(ctx, name, scope, repository, branch, sourceUrl, filename, variables)
			}
			return nil, translatePreviewAPIErr(err, name)
		}
		previewResponse.Action = &types.Action{
			ID:     string(mutation.Preview.Action.Id),
			Name:   string(mutation.Preview.Action.Name),
			Status: string(mutation.Preview.Action.Status),
		}
		previewResponse.Preview = &types.Preview{
			ID: string(mutation.Preview.Preview.Id),
		}
	}
	return previewResponse, nil
}

//TODO: remove when all users are in Okteto Enterprise >= 0.10.0
func (c *OktetoClient) deprecatedDeployPreview(ctx context.Context, name, scope, repository, branch, sourceUrl, filename string, variables []types.Variable) (*types.PreviewResponse, error) {

	previewEnv := &types.PreviewResponse{}

	if len(variables) > 0 {
		var mutation struct {
			Preview struct {
				Id graphql.String
			} `graphql:"deployPreview(name: $name, scope: $scope, repository: $repository, branch: $branch, sourceUrl: $sourceURL, variables: $variables, filename: $filename)"`
		}

		variablesVariable := make([]InputVariable, 0)
		for _, v := range variables {
			variablesVariable = append(variablesVariable, InputVariable{
				Name:  graphql.String(v.Name),
				Value: graphql.String(v.Value),
			})
		}
		variables := map[string]interface{}{
			"name":       graphql.String(name),
			"scope":      PreviewScope(scope),
			"repository": graphql.String(repository),
			"branch":     graphql.String(branch),
			"sourceURL":  graphql.String(sourceUrl),
			"variables":  variablesVariable,
			"filename":   graphql.String(filename),
		}
		err := mutate(ctx, &mutation, variables, c.client)
		if err != nil {
			return nil, translatePreviewAPIErr(err, name)
		}
		previewEnv.Preview = &types.Preview{ID: string(mutation.Preview.Id)}
	} else {
		var mutation struct {
			Preview struct {
				Id graphql.String
			} `graphql:"deployPreview(name: $name, scope: $scope, repository: $repository, branch: $branch, sourceUrl: $sourceURL, filename: $filename)"`
		}

		variables := map[string]interface{}{
			"name":       graphql.String(name),
			"scope":      PreviewScope(scope),
			"repository": graphql.String(repository),
			"branch":     graphql.String(branch),
			"sourceURL":  graphql.String(sourceUrl),
			"filename":   graphql.String(filename),
		}
		err := mutate(ctx, &mutation, variables, c.client)
		if err != nil {
			return nil, translatePreviewAPIErr(err, name)
		}
		previewEnv.Preview = &types.Preview{ID: string(mutation.Preview.Id)}
	}

	return previewEnv, nil
}

// DestroyPreview destroy a preview environment
func (c *OktetoClient) DestroyPreview(ctx context.Context, name string) error {
	var mutation struct {
		Preview struct {
			Id graphql.String
		} `graphql:"destroyPreview(id: $id)"`
	}
	variables := map[string]interface{}{
		"id": graphql.String(name),
	}

	err := mutate(ctx, &mutation, variables, c.client)
	if oktetoErrors.IsNotFound(err) {
		return nil
	}
	return err
}

// ListPreviews list preview environments
func (c *previewClient) List(ctx context.Context) ([]types.Preview, error) {
	var queryStruct struct {
		PreviewEnvs []struct {
			Id       graphql.String
			Sleeping graphql.Boolean
			Scope    graphql.String
		} `graphql:"previews"`
	}

	err := query(ctx, &queryStruct, nil, c.client)
	if err != nil {
		return nil, err
	}

	result := make([]types.Preview, 0)
	for _, previewEnv := range queryStruct.PreviewEnvs {
		result = append(result, types.Preview{
			ID:       string(previewEnv.Id),
			Sleeping: bool(previewEnv.Sleeping),
			Scope:    string(previewEnv.Scope),
		})
	}

	return result, nil
}

func (c *OktetoClient) ListPreviewsEndpoints(ctx context.Context, previewName string) ([]types.Endpoint, error) {
	var queryStruct struct {
		Preview struct {
			Deployments []struct {
				Endpoints []struct {
					Url graphql.String
				}
			}
			Statefulsets []struct {
				Endpoints []struct {
					Url graphql.String
				}
			}
		} `graphql:"preview(id: $id)"`
	}

	variables := map[string]interface{}{
		"id": graphql.String(previewName),
	}
	endpoints := make([]types.Endpoint, 0)

	err := query(ctx, &queryStruct, variables, c.client)
	if err != nil {
		return nil, err
	}

	for _, d := range queryStruct.Preview.Deployments {
		for _, endpoint := range d.Endpoints {
			endpoints = append(endpoints, types.Endpoint{
				URL: string(endpoint.Url),
			})
		}
	}

	for _, sfs := range queryStruct.Preview.Statefulsets {
		for _, endpoint := range sfs.Endpoints {
			endpoints = append(endpoints, types.Endpoint{
				URL: string(endpoint.Url),
			})
		}
	}
	return endpoints, nil
}

// GetPreviewEnvByName gets a preview environment given its name
func (c *OktetoClient) GetPreviewEnvByName(ctx context.Context, name string) (*types.GitDeploy, error) {
	var queryStruct struct {
		Preview struct {
			GitDeploys []struct {
				Id     graphql.String
				Name   graphql.String
				Status graphql.String
			}
		} `graphql:"preview(id: $id)"`
	}

	variables := map[string]interface{}{
		"id": graphql.String(Context().Namespace),
	}
	err := query(ctx, &queryStruct, variables, c.client)
	if err != nil {
		return nil, err
	}

	for _, gitDeploy := range queryStruct.Preview.GitDeploys {
		if string(gitDeploy.Name) == name {
			pipeline := &types.GitDeploy{
				ID:     string(gitDeploy.Id),
				Name:   string(gitDeploy.Name),
				Status: string(gitDeploy.Status),
			}
			return pipeline, nil
		}
	}

	return nil, oktetoErrors.ErrNotFound
}

func (c *OktetoClient) GetResourcesStatusFromPreview(ctx context.Context, previewName string) (map[string]string, error) {
	var queryStruct struct {
		Preview struct {
			Deployments []struct {
				Name   graphql.String
				Status graphql.String
			}
			Statefulsets []struct {
				Name   graphql.String
				Status graphql.String
			}
		} `graphql:"preview(id: $id)"`
	}
	variables := map[string]interface{}{
		"id": graphql.String(previewName),
	}

	err := query(ctx, &queryStruct, variables, c.client)
	if err != nil {
		return nil, err
	}

	status := make(map[string]string)

	for _, d := range queryStruct.Preview.Deployments {
		status[string(d.Name)] = string(d.Status)
	}

	for _, sfs := range queryStruct.Preview.Statefulsets {
		status[string(sfs.Name)] = string(sfs.Status)
	}
	return status, nil
}

func translatePreviewAPIErr(err error, name string) error {
	if err.Error() == "conflict" {
		return fmt.Errorf("preview '%s' already exists with a different scope. Please use a different name", name)
	}
	if strings.Contains(err.Error(), "operation-not-permitted") {
		return oktetoErrors.UserError{E: fmt.Errorf("you are not authorized to create a global preview env"),
			Hint: "Please log in with an administrator account or use a personal preview environment"}
	}
	return err
}
