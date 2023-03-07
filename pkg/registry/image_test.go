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
)

func Test_GetResgistryAndRepo(t *testing.T) {
	var tests = []struct {
		name             string
		image            string
		expectedRegistry string
		expectedRepo     string
	}{
		{
			name:             "official-with-tag",
			image:            "ubuntu:2",
			expectedRegistry: "docker.io",
			expectedRepo:     "ubuntu:2",
		},
		{
			name:             "official-without-tag",
			image:            "ubuntu",
			expectedRegistry: "docker.io",
			expectedRepo:     "ubuntu",
		},
		{
			name:             "repo-with-tag",
			image:            "test/ubuntu:2",
			expectedRegistry: "docker.io",
			expectedRepo:     "test/ubuntu:2",
		},
		{
			name:             "repo-without-tag",
			image:            "test/ubuntu",
			expectedRegistry: "docker.io",
			expectedRepo:     "test/ubuntu",
		},
		{
			name:             "registry-with-tag",
			image:            "registry/gitlab.com/test/ubuntu:2",
			expectedRegistry: "registry/gitlab.com",
			expectedRepo:     "test/ubuntu:2",
		},
		{
			name:             "registry-without-tag",
			image:            "okteto.dev/test/ubuntu",
			expectedRegistry: "okteto.dev",
			expectedRepo:     "test/ubuntu",
		},
		{
			name:             "okteto-registry-only-two",
			image:            "okteto.dev/ubuntu",
			expectedRegistry: "okteto.dev",
			expectedRepo:     "ubuntu",
		},
		{
			name:             "official-with-registry",
			image:            "docker.io/ubuntu",
			expectedRegistry: "docker.io",
			expectedRepo:     "ubuntu",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry, image := GetRegistryAndRepo(tt.image)
			if tt.expectedRepo != image {
				t.Errorf("expected repo %s got %s in test %s", tt.expectedRepo, image, tt.name)
			}
			if tt.expectedRegistry != registry {
				t.Errorf("expected registry %s got %s in test %s", tt.expectedRegistry, registry, tt.name)
			}
		})
	}
}
