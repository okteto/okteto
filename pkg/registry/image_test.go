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

package registry

import (
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
)

type fakeImageConfig struct {
	registryURL string
	isOkteto    bool
	globalNs    string
	ns          string
}

func (f fakeImageConfig) GetRegistryURL() string     { return f.registryURL }
func (f fakeImageConfig) IsOktetoCluster() bool      { return f.isOkteto }
func (f fakeImageConfig) GetGlobalNamespace() string { return f.globalNs }
func (f fakeImageConfig) GetNamespace() string       { return f.ns }

func TestExpandRegistry(t *testing.T) {
	type input struct {
		config fakeImageConfig
		image  string
	}
	var tests = []struct {
		name     string
		input    input
		expected string
	}{
		{
			name: "no need to expand registry - Vanilla",
			input: input{
				config: fakeImageConfig{
					isOkteto: false,
				},
				image: "okteto/okteto:latest",
			},
			expected: "okteto/okteto:latest",
		},
		{
			name: "no need to expand registry - Okteto",
			input: input{
				config: fakeImageConfig{
					isOkteto: true,
				},
				image: "okteto/okteto:latest",
			},
			expected: "okteto/okteto:latest",
		},
		{
			name: "okteto dev should expansion - Okteto",
			input: input{
				config: fakeImageConfig{
					isOkteto:    true,
					ns:          "test",
					registryURL: "https://my-registry",
				},
				image: "okteto.dev/okteto:latest",
			},
			expected: "https://my-registry/test/okteto:latest",
		},
		{
			name: "no need to expand registry - Okteto",
			input: input{
				config: fakeImageConfig{
					isOkteto: false,
				},
				image: "okteto.dev/okteto:latest",
			},
			expected: "okteto.dev/okteto:latest",
		},
		{
			name: "okteto global should expansion - Okteto",
			input: input{
				config: fakeImageConfig{
					isOkteto:    true,
					globalNs:    "test",
					registryURL: "https://my-registry",
				},
				image: "okteto.global/okteto:latest",
			},
			expected: "https://my-registry/test/okteto:latest",
		},
		{
			name: "no need to expand registry - Okteto",
			input: input{
				config: fakeImageConfig{
					isOkteto: false,
				},
				image: "okteto.global/okteto:latest",
			},
			expected: "okteto.global/okteto:latest",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			iCtrl := NewImageCtrl(tt.input.config)
			image := iCtrl.expandImageRegistries(tt.input.image)
			assert.Equal(t, tt.expected, image)
		})
	}
}

func Test_GetRegistryAndRepo(t *testing.T) {
	type expected struct {
		registry string
		image    string
	}
	var tests = []struct {
		name     string
		tag      string
		expected expected
	}{
		{
			name: "is-splitted-image-not-docker-io",
			tag:  "registry.url.net/image",
			expected: expected{
				registry: "registry.url.net",
				image:    "image",
			},
		},
		{
			name: "is-splitted-image-not-docker-io-double-slash",
			tag:  "registry.url.net/image/other",
			expected: expected{
				registry: "registry.url.net",
				image:    "image/other",
			},
		},
		{
			name: "is-splitted-image-docker",
			tag:  "docker.io/image",
			expected: expected{
				registry: "docker.io",
				image:    "image",
			},
		},
		{
			name: "is-splitted-image-docker",
			tag:  "image",
			expected: expected{
				registry: "docker.io",
				image:    "image",
			},
		},
		{
			name: "is-splitted-image-docker",
			tag:  "image/test",
			expected: expected{
				registry: "docker.io",
				image:    "image/test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			iCtrl := ImageCtrl{}
			registry, image := iCtrl.GetRegistryAndRepo(tt.tag)
			assert.Equal(t, tt.expected.registry, registry)
			assert.Equal(t, tt.expected.image, image)
		})
	}
}

func Test_GetRepoNameAndTag(t *testing.T) {
	type expected struct {
		repo string
		tag  string
	}
	var tests = []struct {
		name     string
		image    string
		expected expected
	}{
		{
			name:  "official-with-tag",
			image: "ubuntu:2",
			expected: expected{
				repo: "ubuntu",
				tag:  "2",
			},
		},
		{
			name:  "official-without-tag",
			image: "ubuntu",
			expected: expected{
				repo: "ubuntu",
				tag:  "latest",
			},
		},
		{
			name:  "repo-with-tag",
			image: "test/ubuntu:2",
			expected: expected{
				repo: "test/ubuntu",
				tag:  "2",
			},
		},
		{
			name:  "repo-without-tag",
			image: "test/ubuntu",
			expected: expected{
				repo: "test/ubuntu",
				tag:  "latest",
			},
		},
		{
			name:  "registry-with-tag",
			image: "registry/gitlab.com/test/ubuntu:2",
			expected: expected{
				repo: "registry/gitlab.com/test/ubuntu",
				tag:  "2",
			},
		},
		{
			name:  "registry-without-tag",
			image: "registry/gitlab.com/test/ubuntu",
			expected: expected{
				repo: "registry/gitlab.com/test/ubuntu",
				tag:  "latest",
			},
		},
		{
			name:  "localhost-with-tag",
			image: "localhost:5000/test/ubuntu:2",
			expected: expected{
				repo: "localhost:5000/test/ubuntu",
				tag:  "2",
			},
		},
		{
			name:  "registry-without-tag",
			image: "localhost:5000/test/ubuntu",
			expected: expected{
				repo: "localhost:5000/test/ubuntu",
				tag:  "latest",
			},
		},
		{
			name:  "sha256",
			image: "pchico83/test@sha256:e78ad0d316485b7dbffa944a92b29ea4fa26d53c63054605c4fb7a8b787a673c",
			expected: expected{
				repo: "pchico83/test",
				tag:  "sha256:e78ad0d316485b7dbffa944a92b29ea4fa26d53c63054605c4fb7a8b787a673c",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, tag := ImageCtrl{}.GetRepoNameAndTag(tt.image)
			assert.Equal(t, tt.expected.repo, repo)
			assert.Equal(t, tt.expected.tag, tag)
		})
	}
}

func TestGetExposedPortsFromCfg(t *testing.T) {
	var tests = []struct {
		name     string
		cfg      *v1.ConfigFile
		expected []Port
	}{
		{
			name:     "cfg is nil",
			cfg:      &v1.ConfigFile{},
			expected: nil,
		},
		{
			name: "cfg is empty",
			cfg: &v1.ConfigFile{Config: v1.Config{
				ExposedPorts: map[string]struct{}{},
			},
			},
			expected: nil,
		},
		{
			name: "cfg-with-ports-one-malformed",
			cfg: &v1.ConfigFile{Config: v1.Config{
				ExposedPorts: map[string]struct{}{
					"8080/tcp":    {},
					"5050":        {},
					"my-port/tcp": {},
				},
			},
			},
			expected: []Port{
				{ContainerPort: 8080, Protocol: apiv1.ProtocolTCP},
			},
		},
		{
			name: "cfg-with-ports",
			cfg: &v1.ConfigFile{Config: v1.Config{
				ExposedPorts: map[string]struct{}{
					"8080/tcp": {},
				},
			},
			},
			expected: []Port{
				{ContainerPort: 8080, Protocol: apiv1.ProtocolTCP},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ports := ImageCtrl{}.getExposedPortsFromCfg(tt.cfg)
			assert.Equal(t, tt.expected, ports)
		})
	}
}

func Test_GetExpandedDevTagFromGlobal(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "is dev image",
			input:    "okteto.dev/my-image:okteto",
			expected: "",
		},
		{
			name:     "is dev image with sha",
			input:    "okteto.dev/my-image@sha256:e78ad0d316485b7dbffa944a92b29ea4fa26d53c63054605c4fb7a8b787a673c",
			expected: "",
		},
		{
			name:     "is not okteto image",
			input:    "mongo:okteto",
			expected: "",
		},
		{
			name:     "is global image",
			input:    "okteto.global/my-image:ffa944a92b29ea4fa26d53c63054605c4fb7a8b787a673c",
			expected: "okteto.dev/my-image:okteto",
		},
		{
			name:     "is global image with sha",
			input:    "okteto.global/my-image@sha256:ffa944a92b29ea4fa26d53c63054605c4fb7a8b787a673c",
			expected: "okteto.dev/my-image:okteto",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetDevTagFromGlobal(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}
