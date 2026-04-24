// Copyright 2025 The Okteto Authors
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

// GitCatalogItem represents a catalog item configured by an Okteto admin.
// Admins define these in the Okteto UI as pre-configured dev environment
// templates so developers can deploy them without extra setup.
type GitCatalogItem struct {
	ID            string                   `json:"id" yaml:"id"`
	Name          string                   `json:"name" yaml:"name"`
	RepositoryURL string                   `json:"repositoryUrl" yaml:"repositoryUrl"`
	Branch        string                   `json:"branch" yaml:"branch"`
	ManifestPath  string                   `json:"manifestPath" yaml:"manifestPath"`
	Variables     []GitCatalogItemVariable `json:"variables" yaml:"variables"`
	ReadOnly      bool                     `json:"readOnly" yaml:"readOnly"`
}

// GitCatalogItemVariable is a default variable configured on a catalog item.
type GitCatalogItemVariable struct {
	Name  string `json:"name" yaml:"name"`
	Value string `json:"value" yaml:"value"`
}

// CatalogDeployOptions holds the parameters for deploying a catalog item.
type CatalogDeployOptions struct {
	CatalogItemID        string
	Name                 string
	Repository           string
	Branch               string
	Filename             string
	Namespace            string
	Variables            []Variable
	RedeployDependencies bool
}
