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
	"strings"

	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/types"
	"github.com/shurcooL/graphql"
)

type previewClient struct {
	client             graphqlClientInterface
	namespaceValidator namespaceValidator
}

func newPreviewClient(client graphqlClientInterface) *previewClient {
	return &previewClient{
		client:             client,
		namespaceValidator: newNamespaceValidator(),
	}
}

// PreviewEnv represents an Okteto preview environment
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

type deployPreviewMutation struct {
	Response deployPreviewResponse `graphql:"deployPreview(name: $name, scope: $scope, repository: $repository, branch: $branch, sourceUrl: $sourceURL, variables: $variables, filename: $filename)"`
}

type destroyPreviewMutation struct {
	Response previewIDStruct `graphql:"destroyPreview(id: $id)"`
}

type listPreviewQuery struct {
	Response []previewEnv `graphql:"previews"`
}

type listPreviewEndpoints struct {
	Response previewEndpoints `graphql:"preview(id: $id)"`
}

type getPreviewResources struct {
	Response previewResourcesStatus `graphql:"preview(id: $id)"`
}

type previewResourcesStatus struct {
	Deployments  []resourceInfo
	Statefulsets []resourceInfo
	Jobs         []resourceInfo
	Cronjobs     []resourceInfo
}

type resourceInfo struct {
	ID         graphql.String
	Name       graphql.String
	Status     graphql.String
	DeployedBy graphql.String
}
type previewEndpoints struct {
	Endpoints    []endpointURL
	Deployments  []deploymentEndpoint
	Statefulsets []statefulsetEdnpoint
	Externals    []externalEndpoints
}

type deploymentEndpoint struct {
	Endpoints []endpointURL
}

type statefulsetEdnpoint struct {
	Endpoints []endpointURL
}
type externalEndpoints struct {
	Endpoints []endpointURL
}

type endpointURL struct {
	Url graphql.String
}

type previewEnv struct {
	Id       graphql.String
	Sleeping graphql.Boolean
	Scope    graphql.String
}

type deployPreviewResponse struct {
	Id     graphql.String
	Action actionStruct
}

type previewIDStruct struct {
	Id graphql.String
}

// DeployPreview creates a preview environment
func (c *previewClient) DeployPreview(ctx context.Context, name, scope, repository, branch, sourceUrl, filename string, variables []types.Variable) (*types.PreviewResponse, error) {
	if err := c.namespaceValidator.validate(name, previewEnvObject); err != nil {
		return nil, err
	}
	origin := config.GetDeployOrigin()
	previewResponse := &types.PreviewResponse{}

	if len(variables) > 0 {
		mutationStruct := deployPreviewMutation{}
		variablesVariable := make([]InputVariable, 0)
		for _, v := range variables {
			variablesVariable = append(variablesVariable, InputVariable{
				Name:  graphql.String(v.Name),
				Value: graphql.String(v.Value),
			})
		}
		// if OKTETO_ORIGIN was provided don't override it
		injectOrigin := true
		for _, v := range variablesVariable {
			if v.Name == "OKTETO_ORIGIN" {
				injectOrigin = false
			}
		}
		if injectOrigin {
			variablesVariable = append(variablesVariable, InputVariable{
				Name:  graphql.String("OKTETO_ORIGIN"),
				Value: graphql.String(origin),
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
		err := mutate(ctx, &mutationStruct, queryVariables, c.client)
		if err != nil {
			return nil, c.translateErr(err, name)
		}
		previewResponse.Action = &types.Action{
			ID:     string(mutationStruct.Response.Action.Id),
			Name:   string(mutationStruct.Response.Action.Name),
			Status: string(mutationStruct.Response.Action.Status),
		}
		previewResponse.Preview = &types.Preview{
			ID: string(mutationStruct.Response.Id),
		}
	} else {
		mutationStruct := deployPreviewMutation{}
		queryVariables := map[string]interface{}{
			"name":       graphql.String(name),
			"scope":      PreviewScope(scope),
			"repository": graphql.String(repository),
			"branch":     graphql.String(branch),
			"sourceURL":  graphql.String(sourceUrl),
			"filename":   graphql.String(filename),
			"variables": []InputVariable{
				{Name: graphql.String("OKTETO_ORIGIN"), Value: graphql.String(origin)},
			},
		}
		err := mutate(ctx, &mutationStruct, queryVariables, c.client)
		if err != nil {
			return nil, c.translateErr(err, name)
		}
		previewResponse.Action = &types.Action{
			ID:     string(mutationStruct.Response.Action.Id),
			Name:   string(mutationStruct.Response.Action.Name),
			Status: string(mutationStruct.Response.Action.Status),
		}
		previewResponse.Preview = &types.Preview{
			ID: string(mutationStruct.Response.Id),
		}
	}
	return previewResponse, nil
}

// DestroyPreview destroy a preview environment
func (c *previewClient) Destroy(ctx context.Context, name string) error {
	mutationStruct := destroyPreviewMutation{}
	variables := map[string]interface{}{
		"id": graphql.String(name),
	}

	err := mutate(ctx, &mutationStruct, variables, c.client)
	return err
}

// ListPreviews list preview environments
func (c *previewClient) List(ctx context.Context) ([]types.Preview, error) {
	queryStruct := listPreviewQuery{}

	err := query(ctx, &queryStruct, nil, c.client)
	if err != nil {
		return nil, err
	}

	result := make([]types.Preview, 0)
	for _, previewEnv := range queryStruct.Response {
		result = append(result, types.Preview{
			ID:       string(previewEnv.Id),
			Sleeping: bool(previewEnv.Sleeping),
			Scope:    string(previewEnv.Scope),
		})
	}

	return result, nil
}

// ListEndpoints lists all the endpoints from a preview environment
func (c *previewClient) ListEndpoints(ctx context.Context, previewName string) ([]types.Endpoint, error) {
	queryStruct := listPreviewEndpoints{}
	variables := map[string]interface{}{
		"id": graphql.String(previewName),
	}
	endpoints := make([]types.Endpoint, 0)

	err := query(ctx, &queryStruct, variables, c.client)
	if err != nil {
		return nil, err
	}

	var endpointsMap = map[graphql.String]bool{}

	for _, endpoint := range queryStruct.Response.Endpoints {
		endpointsMap[endpoint.Url] = true
	}

	// @francisco - 2023//05/02
	// The backend Endpoints field was being return empty in okteto clusters <1.9
	// Lets make sure we correctly resolve the endpoints from the known resources
	// All this below should be safe to remove in newer versions of the okteto cli
	// <-- LEGACY START -->
	for _, d := range queryStruct.Response.Deployments {
		for _, endpoint := range d.Endpoints {
			endpointsMap[endpoint.Url] = true
		}
	}
	for _, sfs := range queryStruct.Response.Statefulsets {
		for _, endpoint := range sfs.Endpoints {
			endpointsMap[endpoint.Url] = true
		}
	}
	for _, ext := range queryStruct.Response.Externals {
		for _, endpoint := range ext.Endpoints {
			endpointsMap[endpoint.Url] = true
		}
	}
	// <-- LEGACY END -->

	for url := range endpointsMap {
		endpoints = append(endpoints, types.Endpoint{
			URL: string(url),
		})
	}
	return endpoints, nil
}

func (c *previewClient) GetResourcesStatus(ctx context.Context, previewName, devName string) (map[string]string, error) {
	queryStruct := getPreviewResources{}
	variables := map[string]interface{}{
		"id": graphql.String(previewName),
	}

	err := query(ctx, &queryStruct, variables, c.client)
	if err != nil {
		return nil, err
	}

	status := make(map[string]string)
	for _, d := range queryStruct.Response.Deployments {
		if devName == "" || string(d.DeployedBy) == devName {
			resourceName := getResourceFullName(Deployment, string(d.Name))
			status[resourceName] = string(d.Status)
		}
	}
	for _, sfs := range queryStruct.Response.Statefulsets {
		if devName == "" || string(sfs.DeployedBy) == devName {
			resourceName := getResourceFullName(StatefulSet, string(sfs.Name))
			status[resourceName] = string(sfs.Status)
		}
	}
	for _, j := range queryStruct.Response.Jobs {
		if devName == "" || string(j.DeployedBy) == devName {
			resourceName := getResourceFullName(Job, string(j.Name))
			status[resourceName] = string(j.Status)
		}
	}
	for _, cj := range queryStruct.Response.Cronjobs {
		if devName == "" || string(cj.DeployedBy) == devName {
			resourceName := getResourceFullName(CronJob, string(cj.Name))
			status[resourceName] = string(cj.Status)
		}
	}
	return status, nil
}

func (*previewClient) translateErr(err error, name string) error {
	if err.Error() == "conflict" {
		return previewConflictErr{name: name}
	}
	if strings.Contains(err.Error(), "operation-not-permitted") {
		return oktetoErrors.UserError{E: ErrUnauthorizedGlobalCreation,
			Hint: "Please log in with an administrator account or use a personal preview environment"}
	}
	return err
}
