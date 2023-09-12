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

package types

// PipelineDeployOptions represents the options to deploy a pipeline
type PipelineDeployOptions struct {
	Name       string
	Repository string
	Branch     string
	Filename   string
	Variables  []Variable
	Namespace  string
	Labels     []string
}

// SpaceBody top body answer
type SpaceBody struct {
	Space Space `json:"space"`
}

// GitDeployResponse represents
type GitDeployResponse struct {
	Action    *Action    `json:"action"`
	GitDeploy *GitDeploy `json:"gitDeploy"`
}

// GitDeploy represents an Okteto pipeline status
type GitDeploy struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Repository string `json:"repository"`
	Status     string `json:"status"`
}

// Space represents the contents of an Okteto Cloud space
type Space struct {
	GitDeploys   []GitDeploy   `json:"gitDeploys"`
	Statefulsets []Statefulset `json:"statefulsets"`
	Deployments  []Deployment  `json:"deployments"`
}

// Variable represents a pipeline variable
type Variable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
