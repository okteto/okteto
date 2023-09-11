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
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/types"
	"github.com/shurcooL/graphql"
)

var (
	ErrDeployPipelineLabelsFeatureNotSupported = oktetoErrors.UserError{
		E: errors.New(`deploying a preview environments with labels requires a more recent version of Okteto.

		Consider removing the "--label" flag, or please upgrade to the latest version`),
		Hint: "For more information and upgrade instructions, please visit our docs at https://www.okteto.com/docs or contact your system administrator.",
	}
)

type pipelineClient struct {
	client        graphqlClientInterface
	url           string
	provideTicker func(time.Duration) *time.Ticker
	provideTimer  func(time.Duration) *time.Timer
}

func newPipelineClient(client graphqlClientInterface, url string) *pipelineClient {
	return &pipelineClient{
		client:        client,
		url:           url,
		provideTicker: time.NewTicker,
		provideTimer:  time.NewTimer,
	}
}

type deployPipelineMutation struct {
	Response deployPipelineResponse `graphql:"deployGitRepository(name: $name, repository: $repository, space: $space, branch: $branch, variables: $variables, filename: $filename)"`
}

type deployPipelineMutationWithLabels struct {
	Response deployPipelineResponse `graphql:"deployGitRepository(name: $name, repository: $repository, space: $space, branch: $branch, variables: $variables, filename: $filename, labels: $labels)"`
}

type getPipelineByNameQuery struct {
	Response getPipelineByNameResponse `graphql:"space(id: $id)"`
}

type destroyPipelineWithVolumesMutation struct {
	Response destroyPipelineResponse `graphql:"destroyGitRepository(name: $name, space: $space, destroyVolumes: $destroyVolumes)"`
}

type deprecatedDestroyPipelineWithVolumesMutation struct {
	Response deprecatedDestroyPipelineResponse `graphql:"destroyGitRepository(name: $name, space: $space, destroyVolumes: $destroyVolumes)"`
}

type destroyPipelineWithoutVolumesMutation struct {
	Response destroyPipelineResponse `graphql:"destroyGitRepository(name: $name, space: $space)"`
}

type deprecatedDestroyPipelineWithoutVolumesMutation struct {
	Response deprecatedDestroyPipelineResponse `graphql:"destroyGitRepository(name: $name, space: $space)"`
}

type getPipelineResources struct {
	Response previewResourcesStatus `graphql:"space(id: $id)"`
}

type deployPipelineResponse struct {
	Action    actionStruct
	GitDeploy gitDeployInfoWithRepoInfo
}

type gitDeployInfoWithRepoInfo struct {
	Id         graphql.String
	Name       graphql.String
	Status     graphql.String
	Repository graphql.String
}

type getPipelineByNameResponse struct {
	GitDeploys []gitDeployInfoIdNameStatus
}

type gitDeployInfoIdNameStatus struct {
	Id     graphql.String
	Name   graphql.String
	Status graphql.String
}

type destroyPipelineResponse struct {
	Action    actionStruct
	GitDeploy gitDeployInfoWithRepoInfo
}

type deprecatedDestroyPipelineResponse struct {
	GitDeploy gitDeployInfoWithRepoInfo
}

// Deploy creates a pipeline
func (c *pipelineClient) Deploy(ctx context.Context, opts types.PipelineDeployOptions) (*types.GitDeployResponse, error) {
	oktetoLog.Infof("deploying pipeline '%s' mutation on %s", opts.Name, opts.Namespace)

	mutationVariables := c.getDeployVariables(opts)
	var response deployPipelineResponse
	if len(opts.Labels) == 0 {
		mutationStruct := &deployPipelineMutation{}
		err := mutate(ctx, mutationStruct, mutationVariables, c.client)
		if err != nil {
			return nil, fmt.Errorf("failed to deploy pipeline: %w", err)
		}
		response = mutationStruct.Response
	} else {
		mutationStruct := &deployPipelineMutationWithLabels{}
		err := mutate(ctx, mutationStruct, mutationVariables, c.client)

		if err != nil {
			if strings.Contains(err.Error(), "Unknown argument \"labels\" on field \"deployGitRepository\" of type \"Mutation\"") {
				return nil, oktetoErrors.UserError{E: ErrDeployPipelineLabelsFeatureNotSupported, Hint: "Please upgrade to the latest version or ask your administrator"}
			}
			return nil, fmt.Errorf("failed to deploy pipeline: %w", err)
		}
		response = mutationStruct.Response
	}

	gitDeployResponse := &types.GitDeployResponse{
		Action: &types.Action{
			ID:     string(response.Action.Id),
			Name:   string(response.Action.Name),
			Status: string(response.Action.Status),
		},
		GitDeploy: &types.GitDeploy{
			ID:         string(response.GitDeploy.Id),
			Name:       string(response.GitDeploy.Name),
			Repository: string(response.GitDeploy.Repository),
			Status:     string(response.GitDeploy.Status),
		},
	}
	return gitDeployResponse, nil
}

func (c *pipelineClient) getDeployVariables(opts types.PipelineDeployOptions) map[string]interface{} {
	variablesVariable := make([]InputVariable, 0)
	for _, v := range opts.Variables {
		variablesVariable = append(variablesVariable, InputVariable{
			Name:  graphql.String(v.Name),
			Value: graphql.String(v.Value),
		})
	}
	injectOrigin := true
	for _, v := range variablesVariable {
		if v.Name == "OKTETO_ORIGIN" {
			injectOrigin = false
		}
	}
	if injectOrigin {
		origin := config.GetDeployOrigin()
		variablesVariable = append(variablesVariable, InputVariable{
			Name:  graphql.String("OKTETO_ORIGIN"),
			Value: graphql.String(origin),
		})
	}
	vars := map[string]interface{}{
		"name":       graphql.String(opts.Name),
		"space":      graphql.String(opts.Namespace),
		"repository": graphql.String(opts.Repository),
		"branch":     graphql.String(opts.Branch),
		"variables":  variablesVariable,
		"filename":   graphql.String(opts.Filename),
	}

	if len(opts.Labels) > 0 {
		labelsVariable := make(labelList, 0)
		for _, l := range opts.Labels {
			labelsVariable = append(labelsVariable, graphql.String(l))
		}
		vars["labels"] = labelsVariable
	}
	return vars
}

// GetByName gets a pipeline given its name
func (c *pipelineClient) GetByName(ctx context.Context, name, namespace string) (*types.GitDeploy, error) {
	oktetoLog.Infof("getting pipeline '%s' in namespace '%s'", name, namespace)
	var queryStruct getPipelineByNameQuery
	variables := map[string]interface{}{
		"id": graphql.String(namespace),
	}
	err := query(ctx, &queryStruct, variables, c.client)
	if err != nil {
		return nil, fmt.Errorf("failed to get pipeline: %w", err)
	}

	for _, gitDeploy := range queryStruct.Response.GitDeploys {
		if string(gitDeploy.Name) == name {
			return &types.GitDeploy{
				ID:     string(gitDeploy.Id),
				Name:   string(gitDeploy.Name),
				Status: string(gitDeploy.Status),
			}, nil
		}
	}
	return nil, oktetoErrors.ErrNotFound
}

// Destroy destroys a pipeline
func (c *pipelineClient) Destroy(ctx context.Context, name, namespace string, destroyVolumes bool) (*types.GitDeployResponse, error) {
	oktetoLog.Infof("destroy pipeline: %s/%s", namespace, name)
	gitDeployResponse := &types.GitDeployResponse{}
	if destroyVolumes {
		var mutation destroyPipelineWithVolumesMutation

		queryVariables := map[string]interface{}{
			"name":           graphql.String(name),
			"destroyVolumes": graphql.Boolean(destroyVolumes),
			"space":          graphql.String(namespace),
		}
		err := mutate(ctx, &mutation, queryVariables, c.client)
		if err != nil {
			if strings.Contains(err.Error(), "Cannot query field \"action\" on type \"GitDeploy\"") {
				return c.deprecatedDestroy(ctx, name, namespace, destroyVolumes)
			}
			return nil, fmt.Errorf("failed to deploy pipeline: %w", err)
		}
		gitDeployResponse.Action = &types.Action{
			ID:     string(mutation.Response.Action.Id),
			Name:   string(mutation.Response.Action.Name),
			Status: string(mutation.Response.Action.Status),
		}
		gitDeployResponse.GitDeploy = &types.GitDeploy{
			ID:         string(mutation.Response.GitDeploy.Id),
			Name:       string(mutation.Response.GitDeploy.Name),
			Repository: string(mutation.Response.GitDeploy.Repository),
			Status:     string(mutation.Response.GitDeploy.Status),
		}
	} else {
		var mutation destroyPipelineWithoutVolumesMutation
		queryVariables := map[string]interface{}{
			"name":  graphql.String(name),
			"space": graphql.String(Context().Namespace),
		}
		err := mutate(ctx, &mutation, queryVariables, c.client)
		if err != nil {
			if strings.Contains(err.Error(), "Cannot query field \"action\" on type \"GitDeploy\"") {
				return c.deprecatedDestroy(ctx, name, namespace, destroyVolumes)
			}
			return nil, fmt.Errorf("failed to deploy pipeline: %w", err)
		}
		gitDeployResponse.Action = &types.Action{
			ID:     string(mutation.Response.Action.Id),
			Name:   string(mutation.Response.Action.Name),
			Status: string(mutation.Response.Action.Status),
		}
		gitDeployResponse.GitDeploy = &types.GitDeploy{
			ID:         string(mutation.Response.GitDeploy.Id),
			Name:       string(mutation.Response.GitDeploy.Name),
			Repository: string(mutation.Response.GitDeploy.Repository),
			Status:     string(mutation.Response.GitDeploy.Status),
		}
	}

	oktetoLog.Infof("destroy pipeline: %+v", gitDeployResponse.GitDeploy.Status)
	return gitDeployResponse, nil
}

// TODO: Remove this function when okteto chart 0.10.8 is no longer supported
func (c *pipelineClient) deprecatedDestroy(ctx context.Context, name, namespace string, destroyVolumes bool) (*types.GitDeployResponse, error) {
	oktetoLog.Infof("destroy pipeline: %s/%s", namespace, name)
	gitDeployResponse := &types.GitDeployResponse{}
	if destroyVolumes {
		var mutation deprecatedDestroyPipelineWithVolumesMutation

		queryVariables := map[string]interface{}{
			"name":           graphql.String(name),
			"destroyVolumes": graphql.Boolean(destroyVolumes),
			"space":          graphql.String(namespace),
		}
		err := mutate(ctx, &mutation, queryVariables, c.client)
		if err != nil {
			return nil, fmt.Errorf("failed to deploy pipeline: %w", err)
		}
		gitDeployResponse.GitDeploy = &types.GitDeploy{
			ID:     string(mutation.Response.GitDeploy.Id),
			Status: string(mutation.Response.GitDeploy.Status),
		}
	} else {
		var mutation deprecatedDestroyPipelineWithoutVolumesMutation
		queryVariables := map[string]interface{}{
			"name":  graphql.String(name),
			"space": graphql.String(namespace),
		}
		err := mutate(ctx, &mutation, queryVariables, c.client)
		if err != nil {
			return nil, fmt.Errorf("failed to deploy pipeline: %w", err)
		}
		gitDeployResponse.GitDeploy = &types.GitDeploy{
			ID:     string(mutation.Response.GitDeploy.Id),
			Status: string(mutation.Response.GitDeploy.Status),
		}
	}

	oktetoLog.Infof("destroy pipeline: %+v", gitDeployResponse.GitDeploy.Status)
	return gitDeployResponse, nil
}

// GetResourcesStatus returns the status of deployments statefulsets and jobs
func (c *pipelineClient) GetResourcesStatus(ctx context.Context, name, namespace string) (map[string]string, error) {
	oktetoLog.Infof("get resource status started for pipeline: %s/%s", namespace, name)
	var queryStruct getPipelineResources
	variables := map[string]interface{}{
		"id": graphql.String(namespace),
	}

	if err := query(ctx, &queryStruct, variables, c.client); err != nil {
		if oktetoErrors.IsNotFound(err) {
			okClient, err := NewOktetoClientFromUrlAndToken(c.url, Context().Token)
			if err != nil {
				return nil, err
			}
			return okClient.Previews().GetResourcesStatus(ctx, namespace, name)
		}
		return nil, fmt.Errorf("failed to get resources status: %w", err)
	}

	status := make(map[string]string)
	for _, d := range queryStruct.Response.Deployments {
		if string(d.DeployedBy) == name {
			resourceName := getResourceFullName(Deployment, string(d.Name))
			status[resourceName] = string(d.Status)

		}
	}
	for _, sfs := range queryStruct.Response.Statefulsets {
		if string(sfs.DeployedBy) == name {
			resourceName := getResourceFullName(StatefulSet, string(sfs.Name))
			status[resourceName] = string(sfs.Status)
		}
	}
	for _, j := range queryStruct.Response.Jobs {
		if string(j.DeployedBy) == name {
			resourceName := getResourceFullName(Job, string(j.Name))
			status[resourceName] = string(j.Status)
		}
	}
	for _, cj := range queryStruct.Response.Cronjobs {
		if string(cj.DeployedBy) == name {
			resourceName := getResourceFullName(CronJob, string(cj.Name))
			status[resourceName] = string(cj.Status)
		}
	}
	return status, nil
}

func getResourceFullName(kind, name string) string {
	return strings.ToLower(fmt.Sprintf("%s/%s", kind, name))
}
