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

package devenvironment

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/pkg/model"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiv1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestInferName(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		getRepositoryURL   func(string) (string, error)
		name               string
		ns                 string
		manifestPath       string
		cwd                string
		oktetoManifestPath string
		expectedName       string
		devEnvs            []runtime.Object
	}{
		{
			name: "without-repository-url",
			getRepositoryURL: func(s string) (string, error) {
				return "", assert.AnError
			},
			devEnvs:      []runtime.Object{},
			ns:           "test",
			manifestPath: "my-manifest/okteto.yml",
			cwd:          "/tmp/my-dev-env",
			expectedName: "my-dev-env",
		},
		{
			name: "without-dev-envs",
			getRepositoryURL: func(s string) (string, error) {
				return "https://github.com/test-user/my-dev-env-repository.git", nil
			},
			devEnvs:      []runtime.Object{},
			ns:           "test",
			manifestPath: "my-manifest/okteto.yml",
			cwd:          "/tmp/my-dev-env",
			expectedName: "my-dev-env-repository",
		},
		{
			name: "without-matching-criteria-dev-envs",
			getRepositoryURL: func(s string) (string, error) {
				return "https://github.com/test-user/my-dev-env-repository.git", nil
			},
			devEnvs:      getDevEnvironmentConfigMaps(),
			ns:           "test",
			manifestPath: "my-manifest/okteto.yml",
			cwd:          "/tmp/my-dev-env",
			expectedName: "my-dev-env-repository",
		},
		{
			name: "with-matching-criteria-for-one-dev-envs",
			getRepositoryURL: func(s string) (string, error) {
				return "https://github.com/test/single-repo.git", nil
			},
			devEnvs:      getDevEnvironmentConfigMaps(),
			ns:           "test",
			manifestPath: "my-manifest/okteto.yml",
			cwd:          "/tmp/my-dev-env",
			expectedName: "single dev name",
		},
		{
			name: "with-matching-criteria-for-multiple-dev-envs",
			getRepositoryURL: func(s string) (string, error) {
				return "https://github.com/test/multiple-repo.git", nil
			},
			devEnvs:      getDevEnvironmentConfigMaps(),
			ns:           "test",
			manifestPath: "my-manifest-multiple/okteto.yml",
			cwd:          "/tmp/my-dev-env",
			expectedName: "multiple dev name 1",
		},
		{
			name: "with-non-matching-criteria-but-default-one-dev-okteto.yml",
			getRepositoryURL: func(s string) (string, error) {
				return "https://github.com/test/multiple-repo.git", nil
			},
			devEnvs:            getDevEnvironmentConfigMaps(),
			ns:                 "test",
			manifestPath:       "",
			cwd:                filepath.Clean("/tmp/my-dev-env"),
			oktetoManifestPath: "okteto.yml",
			expectedName:       "single dev name with okteto.yml filename saved",
		},
		{
			name: "with-non-matching-criteria-but-default-one-dev-okteto.yaml",
			getRepositoryURL: func(s string) (string, error) {
				return "https://github.com/test/multiple-repo.git", nil
			},
			devEnvs:            getDevEnvironmentConfigMaps(),
			ns:                 "test",
			manifestPath:       "",
			cwd:                filepath.Clean("/tmp/my-dev-env"),
			oktetoManifestPath: "okteto.yaml",
			expectedName:       "single dev name with okteto.yaml filename saved",
		},
		{
			name: "with-non-matching-criteria-but-default-one-dev-.okteto/okteto.yml",
			getRepositoryURL: func(s string) (string, error) {
				return "https://github.com/test/multiple-repo.git", nil
			},
			devEnvs:            getDevEnvironmentConfigMaps(),
			ns:                 "test",
			manifestPath:       "",
			cwd:                "/tmp/my-dev-env",
			oktetoManifestPath: ".okteto/okteto.yml",
			expectedName:       "single dev name with .okteto/okteto.yml filename saved",
		},
		{
			name: "with-non-matching-criteria-but-default-one-dev-.okteto/okteto.yaml",
			getRepositoryURL: func(s string) (string, error) {
				return "https://github.com/test/multiple-repo.git", nil
			},
			devEnvs:            getDevEnvironmentConfigMaps(),
			ns:                 "test",
			manifestPath:       "",
			cwd:                "/tmp/my-dev-env",
			oktetoManifestPath: ".okteto/okteto.yaml",
			expectedName:       "single dev name with .okteto/okteto.yaml filename saved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewSimpleClientset(tt.devEnvs...)
			inferer := NameInferer{
				k8s:              c,
				getRepositoryURL: tt.getRepositoryURL,
				fs:               afero.NewMemMapFs(),
			}

			_, err := inferer.fs.Create(filepath.Clean(filepath.Join(tt.cwd, tt.oktetoManifestPath)))
			assert.NoError(t, err)
			result := inferer.InferName(ctx, tt.cwd, tt.ns, tt.manifestPath)
			require.Equal(t, tt.expectedName, result)
		})
	}
}

func TestInferNameFromDevEnvsAndRepository(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name          string
		repositoryURL string
		ns            string
		manifestPath  string
		expectedName  string
		devEnvs       []runtime.Object
	}{
		{
			name:          "without-dev-envs",
			repositoryURL: "https://github.com/test-user/my-dev-env-repository.git",
			devEnvs:       []runtime.Object{},
			ns:            "test",
			manifestPath:  "my-manifest/okteto.yml",
			expectedName:  "my-dev-env-repository",
		},
		{
			name:          "without-matching-criteria-dev-envs",
			repositoryURL: "https://github.com/test-user/my-dev-env-repository.git",
			devEnvs:       getDevEnvironmentConfigMaps(),
			ns:            "test",
			manifestPath:  "my-manifest/okteto.yml",
			expectedName:  "my-dev-env-repository",
		},
		{
			name:          "with-matching-criteria-for-one-dev-envs",
			repositoryURL: "https://github.com/test/single-repo.git",
			devEnvs:       getDevEnvironmentConfigMaps(),
			ns:            "test",
			manifestPath:  "my-manifest/okteto.yml",
			expectedName:  "single dev name",
		},
		{
			name:          "with-matching-criteria-for-multiple-dev-envs",
			repositoryURL: "https://github.com/test/multiple-repo.git",
			devEnvs:       getDevEnvironmentConfigMaps(),
			ns:            "test",
			manifestPath:  "my-manifest-multiple/okteto.yml",
			expectedName:  "multiple dev name 1",
		},
		{
			name:          "without-matching-criteria-but-default-one-dev",
			repositoryURL: "https://github.com/test/multiple-repo.git",
			devEnvs:       getDevEnvironmentConfigMaps(),
			ns:            "test",
			manifestPath:  "okteto.yml",
			expectedName:  "single dev name with okteto.yml filename saved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewSimpleClientset(tt.devEnvs...)
			inferer := NameInferer{
				k8s: c,
				getRepositoryURL: func(s string) (string, error) {
					return "", assert.AnError
				},
			}

			result := inferer.InferNameFromDevEnvsAndRepository(ctx, tt.repositoryURL, tt.ns, tt.manifestPath, "")
			require.Equal(t, tt.expectedName, result)
		})
	}
}

func getDevEnvironmentConfigMaps() []runtime.Object {
	return []runtime.Object{
		&apiv1.ConfigMap{
			ObjectMeta: v1.ObjectMeta{
				Name:      "single-dev-name",
				Namespace: "test",
				Labels: map[string]string{
					model.GitDeployLabel: "true",
				},
			},
			Data: map[string]string{
				"name":       "single dev name",
				"repository": "https://github.com/test/single-repo.git",
				"filename":   "my-manifest/okteto.yml",
			},
		},
		&apiv1.ConfigMap{
			ObjectMeta: v1.ObjectMeta{
				Name:      "multiple-dev-name-1",
				Namespace: "test",
				Labels: map[string]string{
					model.GitDeployLabel: "true",
				},
			},
			Data: map[string]string{
				"name":       "multiple dev name 1",
				"repository": "https://github.com/test/multiple-repo.git",
				"filename":   "my-manifest-multiple/okteto.yml",
			},
		},
		&apiv1.ConfigMap{
			ObjectMeta: v1.ObjectMeta{
				Name:      "multiple-dev-name-2",
				Namespace: "test",
				Labels: map[string]string{
					model.GitDeployLabel: "true",
				},
			},
			Data: map[string]string{
				"name":       "multiple dev name 2",
				"repository": "https://github.com/test/multiple-repo.git",
				"filename":   "my-manifest-multiple/okteto.yml",
			},
		},
		&apiv1.ConfigMap{
			ObjectMeta: v1.ObjectMeta{
				Name:      "multiple-dev-name-3",
				Namespace: "test",
				Labels: map[string]string{
					model.GitDeployLabel: "true",
				},
			},
			Data: map[string]string{
				"name":       "multiple dev name 3",
				"repository": "https://github.com/test/multiple-repo.git",
				"filename":   "my-manifest-multiple/okteto.yml",
			},
		},
		&apiv1.ConfigMap{
			ObjectMeta: v1.ObjectMeta{
				Name:      "single-dev-name-with-okteto.yml-name-saved",
				Namespace: "test",
				Labels: map[string]string{
					model.GitDeployLabel: "true",
				},
			},
			Data: map[string]string{
				"name":       "single dev name with okteto.yml filename saved",
				"repository": "https://github.com/test/multiple-repo.git",
				"filename":   "okteto.yml",
			},
		},
		&apiv1.ConfigMap{
			ObjectMeta: v1.ObjectMeta{
				Name:      "single-dev-name-with-okteto.yaml-filename-saved",
				Namespace: "test",
				Labels: map[string]string{
					model.GitDeployLabel: "true",
				},
			},
			Data: map[string]string{
				"name":       "single dev name with okteto.yaml filename saved",
				"repository": "https://github.com/test/multiple-repo.git",
				"filename":   "okteto.yaml",
			},
		},
		&apiv1.ConfigMap{
			ObjectMeta: v1.ObjectMeta{
				Name:      "single-dev-name-with-.okteto-okteto.yml-filename-saved",
				Namespace: "test",
				Labels: map[string]string{
					model.GitDeployLabel: "true",
				},
			},
			Data: map[string]string{
				"name":       "single dev name with .okteto/okteto.yml filename saved",
				"repository": "https://github.com/test/multiple-repo.git",
				"filename":   ".okteto/okteto.yml",
			},
		},
		&apiv1.ConfigMap{
			ObjectMeta: v1.ObjectMeta{
				Name:      "single-dev-name-with-.okteto-okteto.yaml-filename-saved",
				Namespace: "test",
				Labels: map[string]string{
					model.GitDeployLabel: "true",
				},
			},
			Data: map[string]string{
				"name":       "single dev name with .okteto/okteto.yaml filename saved",
				"repository": "https://github.com/test/multiple-repo.git",
				"filename":   ".okteto/okteto.yaml",
			},
		},
	}
}
