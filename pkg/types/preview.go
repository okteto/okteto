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

// Preview represents an Okteto preview environment
type Preview struct {
	ID            string        `json:"id" yaml:"id"`
	Scope         string        `json:"scope" yaml:"scope"`
	Branch        string        `json:"branch" yaml:"branch"`
	GitDeploys    []GitDeploy   `json:"gitDeploys"`
	Statefulsets  []Statefulset `json:"statefulsets"`
	Deployments   []Deployment  `json:"deployments"`
	PreviewLabels []string      `json:"previewLabels" yaml:"previewLabels"`
	Sleeping      bool          `json:"sleeping" yaml:"sleeping"`
}

// PreviewResponse represents the response of a deployPreview
type PreviewResponse struct {
	Action  *Action  `json:"action" yaml:"action"`
	Preview *Preview `json:"preview" yaml:"preview"`
}

// Statefulset represents an Okteto statefulset
type Statefulset struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Status     string     `json:"status"`
	DeployedBy string     `json:"deployedBy"`
	Endpoints  []Endpoint `json:"endpoints"`
}

// Deployment represents an Okteto statefulset
type Deployment struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Status     string     `json:"status"`
	DeployedBy string     `json:"deployedBy"`
	Endpoints  []Endpoint `json:"endpoints"`
}

// Endpoint represents an okteto endpoint
type Endpoint struct {
	URL     string `json:"url"`
	Private bool   `json:"private"`
}
