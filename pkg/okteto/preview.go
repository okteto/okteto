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
)

// DeployPreviewBody top body answer
type DeployPreviewBody struct {
	PreviewEnviroment PreviewEnv `json:"deployPreview" yaml:"deployPreview"`
}

// PreviewBody top body answer
type PreviewBody struct {
	Preview Preview `json:"preview"`
}

// Preview represents the contents of an Okteto Cloud space
type Preview struct {
	GitDeploys   []PipelineRun `json:"gitDeploys"`
	Statefulsets []Statefulset `json:"statefulsets"`
	Deployments  []Deployment  `json:"deployments"`
}

//Statefulset represents an Okteto statefulset
type Statefulset struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Status     string     `json:"status"`
	DeployedBy string     `json:"deployedBy"`
	Endpoints  []Endpoint `json:"endpoints"`
}

//Deployment represents an Okteto statefulset
type Deployment struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Status     string     `json:"status"`
	DeployedBy string     `json:"deployedBy"`
	Endpoints  []Endpoint `json:"endpoints"`
}

//Endpoint represents an okteto endpoint
type Endpoint struct {
	URL     string `json:"url"`
	Private bool   `json:"private"`
	Divert  bool   `json:"divert"`
}

//Previews represents an Okteto list of spaces
type Previews struct {
	Previews []PreviewEnv `json:"previews" yaml:"previews"`
}

//PreviewEnv represents an Okteto preview environment
type PreviewEnv struct {
	ID       string `json:"id" yaml:"id"`
	Sleeping bool   `json:"sleeping" yaml:"sleeping"`
	Scope    string `json:"scope" yaml:"scope"`
}

// CreatePreview creates a preview environment
func DeployPreview(ctx context.Context, name, scope, repository, branch, sourceUrl, filename string, variables []Variable) (string, error) {
	var body DeployPreviewBody
	filenameParameter := ""
	if filename != "" {
		filenameParameter = fmt.Sprintf(`, filename: "%s"`, filename)
	}

	if len(variables) > 0 {
		q := fmt.Sprintf(`mutation deployPreview($variables: [InputVariable]){
			deployPreview(name: "%s, "scope: %s, repository: "%s", branch: "%s", sourceUrl: "%s", variables: $variables%s){
				id
			},
		}`, name, scope, repository, branch, sourceUrl, filenameParameter)

		req := graphql.NewRequest(q)
		req.Var("variables", variables)
		if err := queryWithRequest(ctx, req, &body); err != nil {
			if strings.Contains(err.Error(), "operation-not-permitted") {
				return "", errors.UserError{E: fmt.Errorf("You are not authorized to create a global preview env."),
					Hint: "Please log in with an administrator account or use a personal preview environment"}
			}
			return "", err
		}
	} else {
		q := fmt.Sprintf(`mutation{
			deployPreview(name: "%s", scope: %s, repository: "%s", branch: "%s", sourceUrl: "%s",%s){
				id
			},
		}`, name, scope, repository, branch, sourceUrl, filenameParameter)

		if err := query(ctx, q, &body); err != nil {
			return "", translatePreviewAPIErr(err, name)
		}
	}

	return body.PreviewEnviroment.ID, nil
}

func translatePreviewAPIErr(err error, name string) error {
	if err.Error() == "conflict" {
		return fmt.Errorf("preview '%s' already exists with a different scope. Please use a different name", name)
	}
	return err
}

// DestroyPreview destroy a preview environment
func DestroyPreview(ctx context.Context, name string) error {
	q := fmt.Sprintf(`mutation{
		destroyPreview(id: "%s"){
			id
		},
	}`, name)

	var body DeleteBody
	return query(ctx, q, &body)
}

// ListPreviews list preview environments
func ListPreviews(ctx context.Context) ([]PreviewEnv, error) {
	q := `query{
		previews{
			id,
			sleeping,
			scope,
		},
	}`

	var body Previews
	if err := query(ctx, q, &body); err != nil {
		return nil, err
	}

	return body.Previews, nil
}

func ListPreviewsEndpoints(ctx context.Context, previewName string) ([]Endpoint, error) {
	endpoints := make([]Endpoint, 0)

	q := fmt.Sprintf(`query{
		preview(id: "%s"){
			deployments{
				endpoints{
					url
				}
			},
			statefulsets{
				endpoints{
					url
				}
			}
		}
	}`, previewName)
	var body PreviewBody
	if err := query(ctx, q, &body); err != nil {
		return nil, err
	}

	for _, d := range body.Preview.Deployments {
		for _, endpoint := range d.Endpoints {
			endpoints = append(endpoints, endpoint)
		}
	}

	for _, sfs := range body.Preview.Statefulsets {
		for _, endpoint := range sfs.Endpoints {
			endpoints = append(endpoints, endpoint)
		}
	}
	return endpoints, nil
}

// GetPreviewEnvByName gets a preview environment given its name
func GetPreviewEnvByName(ctx context.Context, name, namespace string) (*PipelineRun, error) {
	q := fmt.Sprintf(`query{
		preview(id: "%s"){
			gitDeploys{
				id,name,status
			}
		},
	}`, namespace)

	var body PreviewBody
	if err := query(ctx, q, &body); err != nil {
		return nil, err
	}

	for _, g := range body.Preview.GitDeploys {
		if g.Name == name {
			return &g, nil
		}
	}

	return nil, errors.ErrNotFound
}

func GetResourcesStatusFromPreview(ctx context.Context, previewName string) (map[string]string, error) {
	status := make(map[string]string)
	q := fmt.Sprintf(`query{
 		preview(id: "%s"){
 			deployments{
 				name, status
 			},
 			statefulsets{
 				name, status
 			}
 		}
 	}`, previewName)
	var body PreviewBody
	if err := query(ctx, q, &body); err != nil {
		return status, err
	}

	for _, d := range body.Preview.Deployments {
		status[d.Name] = d.Status
	}

	for _, sfs := range body.Preview.Statefulsets {
		status[sfs.Name] = sfs.Status
	}
	return status, nil
}
