// Copyright 2020 The Okteto Authors
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

	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
)

func Test_GetRepoNameAndTag(t *testing.T) {
	var tests = []struct {
		name         string
		image        string
		expectedRepo string
		expectedTag  string
	}{
		{
			name:         "official-with-tag",
			image:        "ubuntu:2",
			expectedRepo: "ubuntu",
			expectedTag:  "2",
		},
		{
			name:         "official-without-tag",
			image:        "ubuntu",
			expectedRepo: "ubuntu",
			expectedTag:  "latest",
		},
		{
			name:         "repo-with-tag",
			image:        "test/ubuntu:2",
			expectedRepo: "test/ubuntu",
			expectedTag:  "2",
		},
		{
			name:         "repo-without-tag",
			image:        "test/ubuntu",
			expectedRepo: "test/ubuntu",
			expectedTag:  "latest",
		},
		{
			name:         "registry-with-tag",
			image:        "registry/gitlab.com/test/ubuntu:2",
			expectedRepo: "registry/gitlab.com/test/ubuntu",
			expectedTag:  "2",
		},
		{
			name:         "registry-without-tag",
			image:        "registry/gitlab.com/test/ubuntu",
			expectedRepo: "registry/gitlab.com/test/ubuntu",
			expectedTag:  "latest",
		},
		{
			name:         "localhost-with-tag",
			image:        "localhost:5000/test/ubuntu:2",
			expectedRepo: "localhost:5000/test/ubuntu",
			expectedTag:  "2",
		},
		{
			name:         "registry-without-tag",
			image:        "localhost:5000/test/ubuntu",
			expectedRepo: "localhost:5000/test/ubuntu",
			expectedTag:  "latest",
		},
		{
			name:         "sha256",
			image:        "pchico83/test@sha256:e78ad0d316485b7dbffa944a92b29ea4fa26d53c63054605c4fb7a8b787a673c",
			expectedRepo: "pchico83/test",
			expectedTag:  "sha256:e78ad0d316485b7dbffa944a92b29ea4fa26d53c63054605c4fb7a8b787a673c",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, tag := GetRepoNameAndTag(tt.image)
			if tt.expectedRepo != repo {
				t.Errorf("expected repo %s got %s in test %s", tt.expectedRepo, repo, tt.name)
			}
			if tt.expectedTag != tag {
				t.Errorf("expected tag %s got %s in test %s", tt.expectedTag, tag, tt.name)
			}
		})
	}
}

func Test_GetImageTag(t *testing.T) {
	var tests = []struct {
		name              string
		image             string
		service           string
		namespace         string
		oktetoRegistryURL string
		expected          string
	}{
		{
			name:              "not-in-okteto",
			image:             "okteto/hello",
			service:           "service",
			namespace:         "namespace",
			oktetoRegistryURL: "",
			expected:          "okteto/hello:okteto",
		},
		{
			name:              "in-okteto-image-in-okteto",
			image:             "okteto.dev/hello",
			service:           "service",
			namespace:         "namespace",
			oktetoRegistryURL: "okteto.dev",
			expected:          "okteto.dev/hello",
		},
		{
			name:              "in-okteto-image-not-in-okteto",
			image:             "okteto/hello",
			service:           "service",
			namespace:         "namespace",
			oktetoRegistryURL: "okteto.dev",
			expected:          "okteto.dev/namespace/service:okteto",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetImageTag(tt.image, tt.service, tt.namespace, tt.oktetoRegistryURL)
			if tt.expected != result {
				t.Errorf("Test '%s': expected %s got %s", tt.name, tt.expected, result)
			}
		})
	}
}

func Test_GetDevImageTag(t *testing.T) {
	var tests = []struct {
		name                string
		dev                 *model.Dev
		imageTag            string
		imageFromDeployment string
		oktetoRegistryURL   string
		expected            string
	}{
		{
			name:                "imageTag-not-in-okteto",
			dev:                 &model.Dev{Name: "dev", Namespace: "ns"},
			imageTag:            "imageTag",
			imageFromDeployment: "",
			oktetoRegistryURL:   "",
			expected:            "imageTag",
		},
		{
			name:                "imageTag-in-okteto",
			dev:                 &model.Dev{Name: "dev", Namespace: "ns"},
			imageTag:            "imageTag",
			imageFromDeployment: "",
			oktetoRegistryURL:   okteto.CloudRegistryURL,
			expected:            "imageTag",
		},
		{
			name:                "default-image-tag",
			dev:                 &model.Dev{Name: "dev", Namespace: "ns"},
			imageTag:            model.DefaultImage,
			imageFromDeployment: "",
			oktetoRegistryURL:   okteto.CloudRegistryURL,
			expected:            "registry.cloud.okteto.net/ns/dev:okteto",
		},
		{
			name:                "okteto",
			dev:                 &model.Dev{Name: "dev", Namespace: "ns"},
			imageTag:            "",
			imageFromDeployment: "",
			oktetoRegistryURL:   okteto.CloudRegistryURL,
			expected:            "registry.cloud.okteto.net/ns/dev:okteto",
		},
		{
			name:                "not-in-okteto",
			dev:                 &model.Dev{Name: "dev", Namespace: "ns"},
			imageTag:            "",
			imageFromDeployment: "okteto/test:2",
			oktetoRegistryURL:   "",
			expected:            "okteto/test:okteto",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetDevImageTag(tt.dev, tt.imageTag, tt.imageFromDeployment, tt.oktetoRegistryURL)
			if tt.expected != result {
				t.Errorf("expected %s got %s in test %s", tt.expected, result, tt.name)
			}
		})
	}
}

func Test_translateCacheHandler(t *testing.T) {
	var tests = []struct {
		name     string
		input    string
		userID   string
		expected string
	}{
		{
			name:     "no-matched",
			input:    "RUN go build",
			userID:   "userid",
			expected: "RUN go build",
		},
		{
			name:     "matched-id-first",
			input:    "RUN --mount=id=1,type=cache,target=/root/.cache/go-build go build",
			userID:   "userid",
			expected: "RUN --mount=id=userid-1,type=cache,target=/root/.cache/go-build go build",
		},
		{
			name:     "matched-id-last",
			input:    "RUN --mount=type=cache,target=/root/.cache/go-build,id=1 go build",
			userID:   "userid",
			expected: "RUN --mount=type=cache,target=/root/.cache/go-build,id=userid-1 go build",
		},
		{
			name:     "matched-noid",
			input:    "RUN --mount=type=cache,target=/root/.cache/go-build go build",
			userID:   "userid",
			expected: "RUN --mount=id=userid,type=cache,target=/root/.cache/go-build go build",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := translateCacheHandler(tt.input, tt.userID)
			if tt.expected != result {
				t.Errorf("expected %s got %s in test %s", tt.expected, result, tt.name)
			}
		})
	}
}
