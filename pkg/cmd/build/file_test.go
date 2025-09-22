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

package build

import (
	"testing"

	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/assert"
)

func Test_translateOktetoRegistryImage(t *testing.T) {
	var tests = []struct {
		name      string
		input     string
		namespace string
		registry  string
		want      string
	}{
		{
			name:      "has-okteto-registry-image-dev",
			input:     "FROM okteto.dev/image",
			namespace: "cindy",
			registry:  "registry.url",
			want:      "FROM registry.url/cindy/image",
		},
		{
			name:      "has-okteto-registry-image-global",
			input:     "FROM okteto.global/image",
			namespace: "cindy",
			registry:  "registry.url",
			want:      "FROM registry.url/okteto/image",
		},
		{
			name:      "not-okteto-registry-image",
			input:     "FROM image",
			namespace: "cindy",
			registry:  "registry.url",
			want:      "FROM image",
		},
		{
			name:      "not-image-line",
			input:     "RUN command",
			namespace: "cindy",
			registry:  "registry.url",
			want:      "RUN command",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			okCtx := &okteto.ContextStateless{
				Store: &okteto.ContextStore{
					Contexts: map[string]*okteto.Context{
						"test": {
							Name:      "test",
							Namespace: tt.namespace,
							UserID:    "user-id",
							Registry:  tt.registry,
						},
					},
					CurrentContext: "test",
				},
			}

			if got := translateOktetoRegistryImage(tt.input, okCtx); got != tt.want {
				t.Errorf("registry.translateOktetoRegistryImage = %v,  want %v", got, tt.want)
			}
		})
	}
}

func TestTranslateCacheHandler(t *testing.T) {
	projectHash := "abc123"

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no RUN command",
			input:    "COPY . .",
			expected: "COPY . .",
		},
		{
			name:     "RUN without cache mount",
			input:    "RUN echo hello",
			expected: "RUN echo hello",
		},
		{
			name:     "RUN with cache mount and id already present",
			input:    "RUN --mount=type=cache,id=mycache,target=/root/.cache pip install -r requirements.txt",
			expected: "RUN --mount=type=cache,id=mycache,target=/root/.cache pip install -r requirements.txt",
		},
		{
			name:     "RUN with cache mount but no id, with target",
			input:    "RUN --mount=type=cache,target=/root/.cache pip install -r requirements.txt",
			expected: "RUN --mount=id=abc123-/root/.cache,type=cache,target=/root/.cache pip install -r requirements.txt",
		},
		{
			name:     "RUN with cache mount but no id, without target",
			input:    "RUN --mount=type=cache pip install -r requirements.txt",
			expected: "RUN --mount=id=abc123,type=cache pip install -r requirements.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := translateCacheHandler(tt.input, projectHash)
			assert.Equal(t, tt.expected, output)
		})
	}
}
