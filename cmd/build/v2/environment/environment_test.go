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

package environment

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
)

type fakeRegistry struct {
	registry map[string]fakeImage
}

type fakeImage struct {
	Registry string
	Repo     string
	Tag      string
	ImageRef string
	Args     []string
}

func newFakeRegistry() fakeRegistry {
	return fakeRegistry{
		registry: make(map[string]fakeImage),
	}
}

func (fr fakeRegistry) AddImageByOpts(opts *types.BuildOptions) error {
	fr.registry[opts.Tag] = fakeImage{Args: opts.BuildArgs}
	return nil
}
func (fr fakeRegistry) AddImageByName(images ...string) error {
	for _, image := range images {
		fr.registry[image] = fakeImage{}
	}
	return nil
}
func (fr fakeRegistry) GetImageReference(image string) (registry.OktetoImageReference, error) {
	ref, err := name.ParseReference(image)
	if err != nil {
		return registry.OktetoImageReference{}, err
	}
	return registry.OktetoImageReference{
		Registry: ref.Context().RegistryStr(),
		Repo:     ref.Context().RepositoryStr(),
		Tag:      ref.Identifier(),
		Image:    image,
	}, nil
}

type fakeOktetoBuilder struct {
	registry fakeRegistry
}

func NewFakeBuilder(registry fakeRegistry) *fakeOktetoBuilder {
	return &fakeOktetoBuilder{
		registry: registry,
	}
}

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

			serviceEnvVarsSetter := NewServiceEnvVarsSetter(io.NewIOController(), registry)
			serviceEnvVarsSetter.SetServiceEnvVars(tt.input.service, tt.input.reference)

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
