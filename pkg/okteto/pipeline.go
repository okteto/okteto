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
	"strings"

	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
	"github.com/shurcooL/graphql"
	giturls "github.com/whilp/git-urls"
)

//PipelineRun represents an Okteto pipeline status
type PipelineRun struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Job        string `json:"job"`
	Repository string `json:"repository"`
	Status     string `json:"status"`
}

// Variable represents a pipeline variable
type Variable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// DeployPipeline creates a pipeline
func (c *OktetoClient) DeployPipeline(ctx context.Context, name, namespace, repository, branch, filename string, variables []Variable) (*PipelineRun, error) {

	pipelineRun := &PipelineRun{}
	if len(variables) > 0 {
		var mutation struct {
			PipelineRun struct {
				Id     graphql.String
				Job    graphql.String
				Status graphql.String
			} `graphql:"deployGitRepository(name: $name, repository: $repository, space: $space, branch: $branch, variables: $variables, filename: $filename)"`
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
			"repository": graphql.String(repository),
			"space":      graphql.String(namespace),
			"branch":     graphql.String(branch),
			"variables":  variablesVariable,
			"filename":   graphql.String(filename),
		}

		err := c.client.Mutate(ctx, &mutation, queryVariables)
		if err != nil {
			if strings.Contains(err.Error(), "Cannot query field \"job\" on type \"GitDeploy\"") {
				return c.deprecatedDeployPipeline(ctx, name, namespace, repository, branch, filename, variables)
			}
			return nil, fmt.Errorf("failed to deploy pipeline: %w", translateAPIErr(err))
		}

		pipelineRun.ID = string(mutation.PipelineRun.Id)
		pipelineRun.Job = string(mutation.PipelineRun.Job)
		pipelineRun.Status = string(mutation.PipelineRun.Status)

	} else {
		var mutation struct {
			PipelineRun struct {
				Id     graphql.String
				Job    graphql.String
				Status graphql.String
			} `graphql:"deployGitRepository(name: $name, repository: $repository, space: $space, branch: $branch, filename: $filename)"`
		}

		queryVariables := map[string]interface{}{
			"name":       graphql.String(name),
			"repository": graphql.String(repository),
			"space":      graphql.String(namespace),
			"branch":     graphql.String(branch),
			"filename":   graphql.String(filename),
		}
		err := c.client.Mutate(ctx, &mutation, queryVariables)
		if err != nil {
			if strings.Contains(err.Error(), "Cannot query field \"job\" on type \"GitDeploy\"") {
				return c.deprecatedDeployPipeline(ctx, name, namespace, repository, branch, filename, variables)
			}
			return nil, fmt.Errorf("failed to deploy pipeline: %w", translateAPIErr(err))
		}
		pipelineRun.ID = string(mutation.PipelineRun.Id)
		pipelineRun.Job = string(mutation.PipelineRun.Job)
		pipelineRun.Status = string(mutation.PipelineRun.Status)
	}

	return pipelineRun, nil
}

//TODO: remove when all users are in Okteto Enterprise >= 0.10.0
func (c *OktetoClient) deprecatedDeployPipeline(ctx context.Context, name, namespace, repository, branch, filename string, variables []Variable) (*PipelineRun, error) {

	pipelineRun := &PipelineRun{}
	if len(variables) > 0 {
		var mutation struct {
			PipelineRun struct {
				Id     graphql.String
				Status graphql.String
			} `graphql:"deployGitRepository(name: $name, repository: $repository, space: $space, branch: $branch, variables: $variables, filename: $filename)"`
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
			"repository": graphql.String(repository),
			"space":      graphql.String(namespace),
			"branch":     graphql.String(branch),
			"variables":  variablesVariable,
			"filename":   graphql.String(filename),
		}

		err := c.client.Mutate(ctx, &mutation, queryVariables)
		if err != nil {
			return nil, fmt.Errorf("failed to deploy pipeline: %w", translateAPIErr(err))
		}

		pipelineRun.ID = string(mutation.PipelineRun.Id)
		pipelineRun.Status = string(mutation.PipelineRun.Status)

	} else {
		var mutation struct {
			PipelineRun struct {
				Id     graphql.String
				Status graphql.String
			} `graphql:"deployGitRepository(name: $name, repository: $repository, space: $space, branch: $branch, filename: $filename)"`
		}

		queryVariables := map[string]interface{}{
			"name":       graphql.String(name),
			"repository": graphql.String(repository),
			"space":      graphql.String(namespace),
			"branch":     graphql.String(branch),
			"filename":   graphql.String(filename),
		}
		err := c.client.Mutate(ctx, &mutation, queryVariables)
		if err != nil {
			return nil, fmt.Errorf("failed to deploy pipeline: %w", translateAPIErr(err))
		}
		pipelineRun.ID = string(mutation.PipelineRun.Id)
		pipelineRun.Status = string(mutation.PipelineRun.Status)
	}

	return pipelineRun, nil
}

// GetPipelineByName gets a pipeline given its name
func (c *OktetoClient) GetPipelineByName(ctx context.Context, name, namespace string) (*PipelineRun, error) {
	var query struct {
		Pipeline struct {
			GitDeploys []struct {
				Id     graphql.String
				Name   graphql.String
				Status graphql.String
			}
		} `graphql:"space(id: $id)"`
	}
	variables := map[string]interface{}{
		"id": graphql.String(namespace),
	}
	err := c.client.Query(ctx, &query, variables)
	if err != nil {
		return nil, translateAPIErr(err)
	}

	for _, gitDeploy := range query.Pipeline.GitDeploys {
		if string(gitDeploy.Name) == name {
			pipeline := &PipelineRun{
				ID:     string(gitDeploy.Id),
				Name:   string(gitDeploy.Name),
				Status: string(gitDeploy.Status),
			}
			return pipeline, nil
		}
	}

	return nil, errors.ErrNotFound
}

// GetPipelineByRepository gets a pipeline given its repo url
func (c *OktetoClient) GetPipelineByRepository(ctx context.Context, namespace, repository string) (*PipelineRun, error) {
	var query struct {
		Pipeline struct {
			GitDeploys []struct {
				Id         graphql.String
				Repository graphql.String
				Status     graphql.String
			}
		} `graphql:"space(id: $id)"`
	}
	variables := map[string]interface{}{
		"id": graphql.String(namespace),
	}
	err := c.client.Query(ctx, &query, variables)
	if err != nil {
		return nil, translateAPIErr(err)
	}

	for _, gitDeploy := range query.Pipeline.GitDeploys {
		if areSameRepository(string(gitDeploy.Repository), repository) {
			pipeline := &PipelineRun{
				ID:         string(gitDeploy.Id),
				Repository: string(gitDeploy.Repository),
				Status:     string(gitDeploy.Status),
			}
			return pipeline, nil
		}
	}
	return nil, errors.ErrNotFound
}

func areSameRepository(repoA, repoB string) bool {
	parsedRepoA, _ := giturls.Parse(repoA)
	parsedRepoB, _ := giturls.Parse(repoB)

	if parsedRepoA.Hostname() != parsedRepoB.Hostname() {
		return false
	}

	//In short SSH URLs like git@github.com:okteto/movies.git, path doesn't start with '/', so we need to remove it
	//in case it exists. It also happens with '.git' suffix. You don't have to specify it, so we remove in both cases
	repoPathA := strings.TrimSuffix(strings.TrimPrefix(parsedRepoA.Path, "/"), ".git")
	repoPathB := strings.TrimSuffix(strings.TrimPrefix(parsedRepoB.Path, "/"), ".git")

	return repoPathA == repoPathB
}

// DestroyPipeline destroys a pipeline
func (c *OktetoClient) DestroyPipeline(ctx context.Context, name, namespace string, destroyVolumes bool) (*PipelineRun, error) {
	log.Infof("destroy pipeline: %s/%s", namespace, name)
	pipelineRun := &PipelineRun{}
	if destroyVolumes {
		var mutation struct {
			PipelineRun struct {
				Id     graphql.String
				Job    graphql.String
				Status graphql.String
			} `graphql:"destroyGitRepository(name: $name, space: $space, destroyVolumes: $destroyVolumes)"`
		}

		queryVariables := map[string]interface{}{
			"name":           graphql.String(name),
			"destroyVolumes": graphql.Boolean(destroyVolumes),
			"space":          graphql.String(namespace),
		}
		err := c.client.Mutate(ctx, &mutation, queryVariables)
		if err != nil {
			if strings.Contains(err.Error(), "Cannot query field \"job\" on type \"GitDeploy\"") {
				return c.deprecatedDestroyPipeline(ctx, name, namespace, destroyVolumes)
			}
			return nil, fmt.Errorf("failed to deploy pipeline: %w", translateAPIErr(err))
		}
		pipelineRun.ID = string(mutation.PipelineRun.Id)
		pipelineRun.Job = string(mutation.PipelineRun.Job)
		pipelineRun.Status = string(mutation.PipelineRun.Status)
	} else {
		var mutation struct {
			PipelineRun struct {
				Id     graphql.String
				Job    graphql.String
				Status graphql.String
			} `graphql:"destroyGitRepository(name: $name, space: $space)"`
		}

		queryVariables := map[string]interface{}{
			"name":  graphql.String(name),
			"space": graphql.String(namespace),
		}
		err := c.client.Mutate(ctx, &mutation, queryVariables)
		if err != nil {
			if strings.Contains(err.Error(), "Cannot query field \"job\" on type \"GitDeploy\"") {
				return c.deprecatedDestroyPipeline(ctx, name, namespace, destroyVolumes)
			}
			return nil, fmt.Errorf("failed to deploy pipeline: %w", translateAPIErr(err))
		}
		pipelineRun.ID = string(mutation.PipelineRun.Id)
		pipelineRun.Job = string(mutation.PipelineRun.Job)
		pipelineRun.Status = string(mutation.PipelineRun.Status)
	}

	log.Infof("destroy pipeline: %+v", pipelineRun.Status)
	return pipelineRun, nil
}

func (c *OktetoClient) deprecatedDestroyPipeline(ctx context.Context, name, namespace string, destroyVolumes bool) (*PipelineRun, error) {
	log.Infof("destroy pipeline: %s/%s", namespace, name)
	pipelineRun := &PipelineRun{}
	if destroyVolumes {
		var mutation struct {
			PipelineRun struct {
				Id     graphql.String
				Status graphql.String
			} `graphql:"destroyGitRepository(name: $name, space: $space, destroyVolumes: $destroyVolumes)"`
		}

		queryVariables := map[string]interface{}{
			"name":           graphql.String(name),
			"destroyVolumes": graphql.Boolean(destroyVolumes),
			"space":          graphql.String(namespace),
		}
		err := c.client.Mutate(ctx, &mutation, queryVariables)
		if err != nil {
			return nil, fmt.Errorf("failed to deploy pipeline: %w", translateAPIErr(err))
		}
		pipelineRun.ID = string(mutation.PipelineRun.Id)
		pipelineRun.Status = string(mutation.PipelineRun.Status)
	} else {
		var mutation struct {
			PipelineRun struct {
				Id     graphql.String
				Status graphql.String
			} `graphql:"destroyGitRepository(name: $name, space: $space)"`
		}

		queryVariables := map[string]interface{}{
			"name":  graphql.String(name),
			"space": graphql.String(namespace),
		}
		err := c.client.Mutate(ctx, &mutation, queryVariables)
		if err != nil {
			if strings.Contains(err.Error(), "Cannot query field \"job\" on type \"GitDeploy\"") {
				return c.deprecatedDestroyPipeline(ctx, name, namespace, destroyVolumes)
			}
			return nil, fmt.Errorf("failed to deploy pipeline: %w", translateAPIErr(err))
		}
		pipelineRun.ID = string(mutation.PipelineRun.Id)
		pipelineRun.Status = string(mutation.PipelineRun.Status)
	}

	log.Infof("destroy pipeline: %+v", pipelineRun.Status)
	return pipelineRun, nil
}

func (c *OktetoClient) GetResourcesStatusFromPipeline(ctx context.Context, name, namespace string) (map[string]string, error) {
	pipeline, err := c.GetPipelineByName(ctx, name, namespace)
	if err != nil {
		return nil, err
	}

	var query struct {
		Space struct {
			Deployments []struct {
				Name       graphql.String
				Status     graphql.String
				DeployedBy graphql.String
			}
			Statefulsets []struct {
				Name       graphql.String
				Status     graphql.String
				DeployedBy graphql.String
			}
		} `graphql:"space(id: $id)"`
	}
	variables := map[string]interface{}{
		"id": graphql.String(namespace),
	}

	err = c.client.Query(ctx, &query, variables)
	if err != nil {
		return nil, translateAPIErr(err)
	}

	status := make(map[string]string)
	for _, d := range query.Space.Deployments {
		if string(d.DeployedBy) == pipeline.ID {
			status[string(d.Name)] = string(d.Status)

		}
	}

	for _, sfs := range query.Space.Statefulsets {
		if string(sfs.DeployedBy) == pipeline.ID {
			status[string(sfs.Name)] = string(sfs.Status)
		}
	}
	return status, nil
}
