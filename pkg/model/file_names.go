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

package model

var (
	// pipelineFiles represents the possible names an okteto pipeline can have
	pipelineFiles = []string{
		"okteto-pipeline.yml",
		"okteto-pipeline.yaml",
		"okteto-pipelines.yml",
		"okteto-pipelines.yaml",
		".okteto/okteto-pipeline.yml",
		".okteto/okteto-pipeline.yaml",
		".okteto/okteto-pipelines.yml",
		".okteto/okteto-pipelines.yaml",
	}
	// composeFiles represents the possible names an okteto compose can have
	composeFiles = []string{
		"okteto-stack.yml",
		"okteto-stack.yaml",
		"stack.yml",
		"stack.yaml",
		".okteto/okteto-stack.yml",
		".okteto/okteto-stack.yaml",
		".okteto/stack.yml",
		".okteto/stack.yaml",

		"okteto-compose.yml",
		"okteto-compose.yaml",
		".okteto/okteto-compose.yml",
		".okteto/okteto-compose.yaml",

		"docker-compose.yml",
		"docker-compose.yaml",
		".okteto/docker-compose.yml",
		".okteto/docker-compose.yaml",
	}
	// oktetoManifestFiles represents the possible names an okteto manifest can have
	oktetoManifestFiles = []string{
		"okteto.yml",
		"okteto.yaml",
		".okteto/okteto.yml",
		".okteto/okteto.yaml",
	}
	// helmChartsSubPaths represents the possible names a helm chart can have
	helmChartsSubPaths = []string{
		"chart",
		"charts",
		"helm/chart",
		"helm/charts",
	}
	// k8sManifestSubPaths represents the possible names a k8s manifest can have
	k8sManifestSubPaths = []string{
		"manifests",
		"manifests.yml",
		"manifests.yaml",
		"kubernetes",
		"kubernetes.yml",
		"kubernetes.yaml",
		"k8s",
		"k8s.yml",
		"k8s.yaml",
	}
)
