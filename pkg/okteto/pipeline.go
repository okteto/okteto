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
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/types"
	"github.com/shurcooL/graphql"
	giturls "github.com/whilp/git-urls"
)

type pipelineClient struct {
	client    *graphql.Client
	sseClient *http.Client

	url string
}

func newPipelineClient(client *graphql.Client, sseClient *http.Client, url string) *pipelineClient {
	return &pipelineClient{
		client:    client,
		sseClient: sseClient,
		url:       url,
	}
}

// Deploy creates a pipeline
func (c *pipelineClient) Deploy(ctx context.Context, opts types.PipelineDeployOptions) (*types.GitDeployResponse, error) {
	origin := config.GetDeployOrigin()

	gitDeployResponse := &types.GitDeployResponse{}

	if len(opts.Variables) > 0 {
		var mutation struct {
			GitDeployResponse struct {
				Action struct {
					Id     graphql.String
					Name   graphql.String
					Status graphql.String
				}
				GitDeploy struct {
					Id         graphql.String
					Name       graphql.String
					Status     graphql.String
					Repository graphql.String
				}
			} `graphql:"deployGitRepository(name: $name, repository: $repository, space: $space, branch: $branch, variables: $variables, filename: $filename)"`
		}

		variablesArg := []InputVariable{}
		for _, v := range opts.Variables {
			variablesArg = append(variablesArg, InputVariable{
				Name:  graphql.String(v.Name),
				Value: graphql.String(v.Value),
			})
		}
		// if OKTETO_ORIGIN was provided don't override it
		injectOrigin := true
		for _, v := range variablesArg {
			if v.Name == "OKTETO_ORIGIN" {
				injectOrigin = false
			}
		}
		if injectOrigin {
			variablesArg = append(variablesArg, InputVariable{
				Name:  graphql.String("OKTETO_ORIGIN"),
				Value: graphql.String(origin),
			})
		}
		queryVariables := map[string]interface{}{
			"name":       graphql.String(opts.Name),
			"repository": graphql.String(opts.Repository),
			"space":      graphql.String(opts.Namespace),
			"branch":     graphql.String(opts.Branch),
			"variables":  variablesArg,
			"filename":   graphql.String(opts.Filename),
		}

		err := mutate(ctx, &mutation, queryVariables, c.client)
		if err != nil {
			return nil, fmt.Errorf("failed to deploy pipeline: %w", err)
		}

		gitDeployResponse.Action = &types.Action{
			ID:     string(mutation.GitDeployResponse.Action.Id),
			Name:   string(mutation.GitDeployResponse.Action.Name),
			Status: string(mutation.GitDeployResponse.Action.Status),
		}
		gitDeployResponse.GitDeploy = &types.GitDeploy{
			ID:         string(mutation.GitDeployResponse.GitDeploy.Id),
			Name:       string(mutation.GitDeployResponse.GitDeploy.Name),
			Repository: string(mutation.GitDeployResponse.GitDeploy.Repository),
			Status:     string(mutation.GitDeployResponse.GitDeploy.Status),
		}

	} else {
		var mutation struct {
			GitDeployResponse struct {
				Action struct {
					Id     graphql.String
					Name   graphql.String
					Status graphql.String
				}
				GitDeploy struct {
					Id         graphql.String
					Name       graphql.String
					Status     graphql.String
					Repository graphql.String
				}
			} `graphql:"deployGitRepository(name: $name, repository: $repository, space: $space, branch: $branch, filename: $filename, variables: $variables)"`
		}

		queryVariables := map[string]interface{}{
			"name":       graphql.String(opts.Name),
			"repository": graphql.String(opts.Repository),
			"space":      graphql.String(opts.Namespace),
			"branch":     graphql.String(opts.Branch),
			"filename":   graphql.String(opts.Filename),
			"variables": []InputVariable{
				{Name: graphql.String("OKTETO_ORIGIN"), Value: graphql.String(origin)},
			},
		}

		err := mutate(ctx, &mutation, queryVariables, c.client)
		if err != nil {
			return nil, fmt.Errorf("failed to deploy pipeline: %w", err)
		}

		gitDeployResponse.Action = &types.Action{
			ID:     string(mutation.GitDeployResponse.Action.Id),
			Name:   string(mutation.GitDeployResponse.Action.Name),
			Status: string(mutation.GitDeployResponse.Action.Status),
		}
		gitDeployResponse.GitDeploy = &types.GitDeploy{
			ID:         string(mutation.GitDeployResponse.GitDeploy.Id),
			Name:       string(mutation.GitDeployResponse.GitDeploy.Name),
			Repository: string(mutation.GitDeployResponse.GitDeploy.Repository),
			Status:     string(mutation.GitDeployResponse.GitDeploy.Status),
		}

	}
	return gitDeployResponse, nil
}

// GetByName gets a pipeline given its name
func (c *pipelineClient) GetByName(ctx context.Context, name, namespace string) (*types.GitDeploy, error) {
	var queryStruct struct {
		Space struct {
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
	err := query(ctx, &queryStruct, variables, c.client)
	if err != nil {
		return nil, err
	}

	for _, gitDeploy := range queryStruct.Space.GitDeploys {
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

// GetByRepository gets a pipeline given its repo url
func (c *pipelineClient) GetByRepository(ctx context.Context, repository string) (*types.GitDeployResponse, error) {
	var queryStruct struct {
		Pipeline struct {
			GitDeploys []struct {
				Id         graphql.String
				Repository graphql.String
				Status     graphql.String
			}
		} `graphql:"space(id: $id)"`
	}
	variables := map[string]interface{}{
		"id": graphql.String(Context().Namespace),
	}
	err := query(ctx, &queryStruct, variables, c.client)
	if err != nil {
		return nil, err
	}

	for _, gitDeploy := range queryStruct.Pipeline.GitDeploys {
		if AreSameRepository(string(gitDeploy.Repository), repository) {
			pipeline := &types.GitDeployResponse{
				GitDeploy: &types.GitDeploy{
					ID:         string(gitDeploy.Id),
					Repository: string(gitDeploy.Repository),
					Status:     string(gitDeploy.Status),
				},
			}
			return pipeline, nil
		}
	}
	return nil, oktetoErrors.ErrNotFound
}

func AreSameRepository(repoA, repoB string) bool {
	parsedRepoA, err := giturls.Parse(repoA)
	if err != nil {
		return false
	}
	parsedRepoB, err := giturls.Parse(repoB)
	if err != nil {
		return false
	}

	if parsedRepoA.Hostname() != parsedRepoB.Hostname() {
		return false
	}

	// In short SSH URLs like git@github.com:okteto/movies.git, path doesn't start with '/', so we need to remove it
	// in case it exists. It also happens with '.git' suffix. You don't have to specify it, so we remove in both cases
	repoPathA := strings.TrimSuffix(strings.TrimPrefix(parsedRepoA.Path, "/"), ".git")
	repoPathB := strings.TrimSuffix(strings.TrimPrefix(parsedRepoB.Path, "/"), ".git")

	return repoPathA == repoPathB
}

// Destroy destroys a pipeline
func (c *pipelineClient) Destroy(ctx context.Context, name string, destroyVolumes bool) (*types.GitDeployResponse, error) {
	oktetoLog.Infof("destroy pipeline: %s/%s", Context().Namespace, name)
	gitDeployResponse := &types.GitDeployResponse{}
	if destroyVolumes {
		var mutation struct {
			GitDeployResponse struct {
				Action struct {
					Id     graphql.String
					Name   graphql.String
					Status graphql.String
				}
				GitDeploy struct {
					Id         graphql.String
					Name       graphql.String
					Status     graphql.String
					Repository graphql.String
				}
			} `graphql:"destroyGitRepository(name: $name, space: $space, destroyVolumes: $destroyVolumes)"`
		}

		queryVariables := map[string]interface{}{
			"name":           graphql.String(name),
			"destroyVolumes": graphql.Boolean(destroyVolumes),
			"space":          graphql.String(Context().Namespace),
		}
		err := mutate(ctx, &mutation, queryVariables, c.client)
		if err != nil {
			if strings.Contains(err.Error(), "Cannot query field \"action\" on type \"GitDeploy\"") {
				return c.deprecatedDestroy(ctx, name, destroyVolumes)
			}
			return nil, fmt.Errorf("failed to deploy pipeline: %w", err)
		}
		gitDeployResponse.Action = &types.Action{
			ID:     string(mutation.GitDeployResponse.Action.Id),
			Name:   string(mutation.GitDeployResponse.Action.Name),
			Status: string(mutation.GitDeployResponse.Action.Status),
		}
		gitDeployResponse.GitDeploy = &types.GitDeploy{
			ID:         string(mutation.GitDeployResponse.GitDeploy.Id),
			Name:       string(mutation.GitDeployResponse.GitDeploy.Name),
			Repository: string(mutation.GitDeployResponse.GitDeploy.Repository),
			Status:     string(mutation.GitDeployResponse.GitDeploy.Status),
		}
	} else {
		var mutation struct {
			GitDeployResponse struct {
				Action struct {
					Id     graphql.String
					Name   graphql.String
					Status graphql.String
				}
				GitDeploy struct {
					Id         graphql.String
					Name       graphql.String
					Status     graphql.String
					Repository graphql.String
				}
			} `graphql:"destroyGitRepository(name: $name, space: $space)"`
		}

		queryVariables := map[string]interface{}{
			"name":  graphql.String(name),
			"space": graphql.String(Context().Namespace),
		}
		err := mutate(ctx, &mutation, queryVariables, c.client)
		if err != nil {
			if strings.Contains(err.Error(), "Cannot query field \"action\" on type \"GitDeploy\"") {
				return c.deprecatedDestroy(ctx, name, destroyVolumes)
			}
			return nil, fmt.Errorf("failed to deploy pipeline: %w", err)
		}
		gitDeployResponse.Action = &types.Action{
			ID:     string(mutation.GitDeployResponse.Action.Id),
			Name:   string(mutation.GitDeployResponse.Action.Name),
			Status: string(mutation.GitDeployResponse.Action.Status),
		}
		gitDeployResponse.GitDeploy = &types.GitDeploy{
			ID:         string(mutation.GitDeployResponse.GitDeploy.Id),
			Name:       string(mutation.GitDeployResponse.GitDeploy.Name),
			Repository: string(mutation.GitDeployResponse.GitDeploy.Repository),
			Status:     string(mutation.GitDeployResponse.GitDeploy.Status),
		}
	}

	oktetoLog.Infof("destroy pipeline: %+v", gitDeployResponse.GitDeploy.Status)
	return gitDeployResponse, nil
}

func (c *pipelineClient) deprecatedDestroy(ctx context.Context, name string, destroyVolumes bool) (*types.GitDeployResponse, error) {
	oktetoLog.Infof("destroy pipeline: %s/%s", Context().Namespace, name)
	gitDeployResponse := &types.GitDeployResponse{}
	if destroyVolumes {
		var mutation struct {
			GitDeployResponse struct {
				Id     graphql.String
				Status graphql.String
			} `graphql:"destroyGitRepository(name: $name, space: $space, destroyVolumes: $destroyVolumes)"`
		}

		queryVariables := map[string]interface{}{
			"name":           graphql.String(name),
			"destroyVolumes": graphql.Boolean(destroyVolumes),
			"space":          graphql.String(Context().Namespace),
		}
		err := mutate(ctx, &mutation, queryVariables, c.client)
		if err != nil {
			return nil, fmt.Errorf("failed to deploy pipeline: %w", err)
		}
		gitDeployResponse.GitDeploy = &types.GitDeploy{
			ID:     string(mutation.GitDeployResponse.Id),
			Status: string(mutation.GitDeployResponse.Status),
		}
	} else {
		var mutation struct {
			GitDeployResponse struct {
				Id     graphql.String
				Status graphql.String
			} `graphql:"destroyGitRepository(name: $name, space: $space)"`
		}

		queryVariables := map[string]interface{}{
			"name":  graphql.String(name),
			"space": graphql.String(Context().Namespace),
		}
		err := mutate(ctx, &mutation, queryVariables, c.client)
		if err != nil {
			if strings.Contains(err.Error(), "Cannot query field \"action\" on type \"GitDeploy\"") {
				return c.deprecatedDestroy(ctx, name, destroyVolumes)
			}
			return nil, fmt.Errorf("failed to deploy pipeline: %w", err)
		}
		gitDeployResponse.GitDeploy = &types.GitDeploy{
			ID:     string(mutation.GitDeployResponse.Id),
			Status: string(mutation.GitDeployResponse.Status),
		}
	}

	oktetoLog.Infof("destroy pipeline: %+v", gitDeployResponse.GitDeploy.Status)
	return gitDeployResponse, nil
}

// GetResourcesStatus returns the status of deployments statefulsets and jobs
func (c *pipelineClient) GetResourcesStatus(ctx context.Context, name, namespace string) (map[string]string, error) {
	var queryStruct struct {
		Space struct {
			Deployments []struct {
				ID         graphql.String
				Name       graphql.String
				Status     graphql.String
				DeployedBy graphql.String
			}
			Statefulsets []struct {
				ID         graphql.String
				Name       graphql.String
				Status     graphql.String
				DeployedBy graphql.String
			}
			Jobs []struct {
				ID         graphql.String
				Name       graphql.String
				Status     graphql.String
				DeployedBy graphql.String
			}
		} `graphql:"space(id: $id)"`
	}
	variables := map[string]interface{}{
		"id": graphql.String(namespace),
	}

	if err := query(ctx, &queryStruct, variables, c.client); err != nil {
		if oktetoErrors.IsNotFound(err) {
			okClient, err := NewOktetoClientFromUrlAndToken(c.url, Context().Token)
			if err != nil {
				return nil, fmt.Errorf("could not create okteto client")
			}
			return okClient.Previews().GetResourcesStatusFromPreview(ctx, namespace, name)
		}
		return nil, err
	}

	status := make(map[string]string)
	for _, d := range queryStruct.Space.Deployments {
		if string(d.DeployedBy) == name {
			resourceName := getResourceFullName(Deployment, string(d.Name))
			status[resourceName] = string(d.Status)

		}
	}
	for _, sfs := range queryStruct.Space.Statefulsets {
		if string(sfs.DeployedBy) == name {
			resourceName := getResourceFullName(StatefulSet, string(sfs.Name))
			status[resourceName] = string(sfs.Status)
		}
	}
	for _, j := range queryStruct.Space.Jobs {
		if string(j.DeployedBy) == name {
			resourceName := getResourceFullName(job, string(j.Name))
			status[resourceName] = string(j.Status)
		}
	}
	return status, nil
}

func getResourceFullName(kind, name string) string {
	return strings.ToLower(fmt.Sprintf("%s/%s", kind, name))
}

// StreamLogs retrieves logs from the pipeline provided and prints them, returns error
func (c *pipelineClient) StreamLogs(ctx context.Context, name, namespace, actionName string) error {
	streamURL := fmt.Sprintf("%s/sse/logs/%s/gitdeploy/%s?action=%s", Context().Name, namespace, name, actionName)
	url, err := url.Parse(streamURL)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return err
	}
	resp, err := c.sseClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error retrieving logs from pipeline %s: %s", name, resp.Status)
	}

	sc := bufio.NewScanner(resp.Body)
	dataHeader := "data: "
	for sc.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			scanText := sc.Text()
			msg := ""

			// if the text scanned is a data message, trim and save into msg
			if strings.HasPrefix(scanText, dataHeader) {
				msg = strings.TrimPrefix(scanText, dataHeader)
			}

			// the msg from sse data is a string with a json log format
			eventLog := &oktetoLog.JSONLogFormat{}
			json.Unmarshal([]byte(msg), &eventLog)

			// skip when the message is empty
			if eventLog.Message == "" {
				continue
			}

			// stop the scanner when the event log is in stage done and message is EOF
			if eventLog.Stage == "done" && eventLog.Message == "EOF" {
				break
			}

			oktetoLog.Println(eventLog.Message)
		}
	}

	// return whether the scan has encountered any error
	return sc.Err()
}
