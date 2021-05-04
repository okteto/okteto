// Copyright 2020 The Okteto Authors
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

	"github.com/machinebox/graphql"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
	giturls "github.com/whilp/git-urls"
)

// DeployPipelineBody top body answer
type DeployPipelineBody struct {
	PipelineRun PipelineRun `json:"createSpace"`
}

// SpaceBody top body answer
type SpaceBody struct {
	Space Space `json:"space"`
}

// DestroyPipelineBody top body answer
type DestroyPipelineBody struct {
	PipelineRun PipelineRun `json:"deleteSpace"`
}

//PipelineRun represents an Okteto pipeline status
type PipelineRun struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Repository string `json:"repository"`
	Status     string `json:"status"`
}

// Space represents the contents of an Okteto Cloud space
type Space struct {
	GitDeploys []PipelineRun `json:"gitDeploys"`
}

// Variable represents a pipeline variable
type Variable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// DeployPipeline creates a pipeline
func DeployPipeline(ctx context.Context, name, namespace, repository, branch string, variables []Variable) (string, error) {
	var body DeployPipelineBody
	if len(variables) > 0 {
		q := fmt.Sprintf(`mutation deployGitRepository($variables: [InputVariable]){
			deployGitRepository(name: "%s", repository: "%s", space: "%s", branch: "%s", variables: $variables){
				id,status
			},
		}`, name, repository, namespace, branch)
		req := graphql.NewRequest(q)
		req.Var("variables", variables)

		if err := queryWithRequest(ctx, req, &body); err != nil {
			return "", err
		}
	} else {
		q := fmt.Sprintf(`mutation{
			deployGitRepository(name: "%s", repository: "%s", space: "%s", branch: "%s"){
				id,status
			},
		}`, name, repository, namespace, branch)

		if err := query(ctx, q, &body); err != nil {
			return "", err
		}
	}

	return body.PipelineRun.Status, nil
}

// GetPipelineByName gets a pipeline given its name
func GetPipelineByName(ctx context.Context, name, namespace string) (*PipelineRun, error) {
	q := fmt.Sprintf(`query{
		space(id: "%s"){
			gitDeploys{
				id,name,status
			}
		},
	}`, namespace)

	var body SpaceBody
	if err := query(ctx, q, &body); err != nil {
		return nil, err
	}

	for _, g := range body.Space.GitDeploys {
		if g.Name == name {
			return &g, nil
		}
	}

	return nil, errors.ErrNotFound
}

// GetPipelineByRepository gets a pipeline given its repo url
func GetPipelineByRepository(ctx context.Context, namespace, repository string) (*PipelineRun, error) {
	q := fmt.Sprintf(`query{
		space(id: "%s"){
			gitDeploys{
				id,repository,status
			}
		},
	}`, namespace)

	var body SpaceBody
	if err := query(ctx, q, &body); err != nil {
		return nil, err
	}

	for _, g := range body.Space.GitDeploys {
		if areSameRepository(g.Repository, repository) {
			return &g, nil
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

// DeletePipeline deletes a pipeline
func DeletePipeline(ctx context.Context, name, namespace string, destroyVolumes bool) (string, error) {
	log.Infof("delete pipeline: %s/%s", namespace, name)
	q := ""
	if destroyVolumes {
		q = fmt.Sprintf(`mutation{
			destroyGitRepository(name: "%s", space: "%s", destroyVolumes: %t){
				id,status
			},
		}`, name, namespace, destroyVolumes)
	} else {
		q = fmt.Sprintf(`mutation{
			destroyGitRepository(name: "%s", space: "%s"){
				id,status
			},
		}`, name, namespace)
	}

	var body DeployPipelineBody
	if err := query(ctx, q, &body); err != nil {
		return "", err
	}

	log.Infof("deleted pipeline: %+v", body.PipelineRun.Status)
	return body.PipelineRun.Status, nil
}
