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

package v2

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
)

func Test_SetServiceEnvVars(t *testing.T) {
	type input struct {
		service   string
		reference string
	}
	type expected struct {
		expRegistry   string
		expRepository string
		expImage      string
		expTag        string
		expSHA        string
	}
	tests := []struct {
		name     string
		input    input
		expected expected
	}{
		{
			name: "setting-variables",
			input: input{
				service:   "frontend",
				reference: "registry.url/namespace/frontend@sha256:7075f1094117e418764bb9b47a5dfc093466e714ec385223fb582d78220c7252",
			},
			expected: expected{
				expRegistry:   "registry.url",
				expRepository: "namespace/frontend",
				expImage:      "registry.url/namespace/frontend@sha256:7075f1094117e418764bb9b47a5dfc093466e714ec385223fb582d78220c7252",
				expTag:        "sha256:7075f1094117e418764bb9b47a5dfc093466e714ec385223fb582d78220c7252",
				expSHA:        "okteto@sha256:7075f1094117e418764bb9b47a5dfc093466e714ec385223fb582d78220c7252",
			},
		},
		{
			name: "setting-variables-no-tag",
			input: input{
				service:   "frontend",
				reference: "registry.url/namespace/frontend",
			},
			expected: expected{
				expRegistry:   "registry.url",
				expRepository: "namespace/frontend",
				expImage:      "registry.url/namespace/frontend",
				expTag:        "latest",
				expSHA:        "latest",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registryEnv := fmt.Sprintf("OKTETO_BUILD_%s_REGISTRY", strings.ToUpper(tt.input.service))
			imageEnv := fmt.Sprintf("OKTETO_BUILD_%s_IMAGE", strings.ToUpper(tt.input.service))
			repositoryEnv := fmt.Sprintf("OKTETO_BUILD_%s_REPOSITORY", strings.ToUpper(tt.input.service))
			tagEnv := fmt.Sprintf("OKTETO_BUILD_%s_TAG", strings.ToUpper(tt.input.service))
			shaEnv := fmt.Sprintf("OKTETO_BUILD_%s_SHA", strings.ToUpper(tt.input.service))

			envs := []string{registryEnv, imageEnv, repositoryEnv, tagEnv}
			for _, e := range envs {
				if err := os.Unsetenv(e); err != nil {
					t.Errorf("error unsetting var %s", err.Error())
				}
			}
			for _, e := range envs {
				if v := os.Getenv(e); v != "" {
					t.Errorf("env variable is already set [%v]", e)
				}
			}

			registry := newFakeRegistry()
			fakeConfig := fakeConfig{
				isOkteto: true,
			}
			bc := NewFakeBuilder(nil, registry, fakeConfig, &fakeAnalyticsTracker{})
			bc.SetServiceEnvVars(tt.input.service, tt.input.reference)

			registryEnvValue := os.Getenv(registryEnv)
			imageEnvValue := os.Getenv(imageEnv)
			repositoryEnvValue := os.Getenv(repositoryEnv)
			tagEnvValue := os.Getenv(tagEnv)
			shaEnvValue := os.Getenv(shaEnv)

			assert.Equal(t, tt.expected.expRegistry, registryEnvValue)
			assert.Equal(t, tt.expected.expImage, imageEnvValue)
			assert.Equal(t, tt.expected.expRepository, repositoryEnvValue)
			assert.Equal(t, tt.expected.expTag, tagEnvValue)
			assert.Equal(t, tt.expected.expSHA, shaEnvValue)
		})
	}
}

func TestExpandStackVariables(t *testing.T) {
	ctx := context.Background()
	okteto.CurrentStore = &okteto.OktetoContextStore{
		Contexts: map[string]*okteto.OktetoContext{
			"test": {
				Namespace: "test",
				IsOkteto:  true,
			},
		},
		CurrentContext: "test",
	}

	registry := newFakeRegistry()
	builder := test.NewFakeOktetoBuilder(registry)
	fakeConfig := fakeConfig{
		isOkteto: true,
	}

	bc := NewFakeBuilder(builder, registry, fakeConfig, &fakeAnalyticsTracker{})
	stack := &model.Stack{
		Services: map[string]*model.Service{
			"test": {
				Image: "{OKTETO_BUILD_TEST_IMAGE}",
			},
		},
	}

	manifest := &model.Manifest{
		Name: "test",
		Build: model.ManifestBuild{
			"test": &model.BuildInfo{
				Image: "nginx",
				VolumesToInclude: []model.StackVolume{
					{
						LocalPath:  "test",
						RemotePath: "test",
					},
				},
			},
		},
		Deploy: &model.DeployInfo{
			ComposeSection: &model.ComposeSectionInfo{
				Stack: stack,
			},
		},
		Type: model.StackType,
		IsV2: true,
	}
	err := bc.Build(ctx, &types.BuildOptions{
		Manifest: manifest,
	})

	// error from the build
	assert.NoError(t, err)

	// Not substituted by empty string
	assert.NotEmpty(t, manifest.Deploy.ComposeSection.Stack.Services["test"].Image)
	assert.NotEqual(t, manifest.Deploy.ComposeSection.Stack.Services["test"].Image, "{OKTETO_BUILD_TEST_IMAGE}")
}
