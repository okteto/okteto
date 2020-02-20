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

package build

import (
	"testing"

	"github.com/okteto/okteto/pkg/okteto"
)

func Test_GetImageTag(t *testing.T) {
	var tests = []struct {
		tname              string
		name               string
		namespace          string
		imageTag           string
		deploymentImageTag string
		oktetoRegistryURL  string
		expected           string
	}{
		{
			tname:              "imageTag-not-in-okteto",
			name:               "dev",
			namespace:          "ns",
			imageTag:           "imageTag",
			deploymentImageTag: "",
			oktetoRegistryURL:  "",
			expected:           "imageTag",
		},
		{
			tname:              "imageTag-in-okteto",
			name:               "dev",
			namespace:          "ns",
			imageTag:           "imageTag",
			deploymentImageTag: "",
			oktetoRegistryURL:  okteto.CloudRegistryURL,
			expected:           "imageTag",
		},
		{
			tname:              "okteto",
			name:               "dev",
			namespace:          "ns",
			imageTag:           "",
			deploymentImageTag: "",
			oktetoRegistryURL:  okteto.CloudRegistryURL,
			expected:           "registry.cloud.okteto.net/ns/dev:okteto-cache",
		},
		{
			tname:              "not-in-okteto",
			name:               "dev",
			namespace:          "ns",
			imageTag:           "",
			deploymentImageTag: "okteto/test:2",
			oktetoRegistryURL:  "",
			expected:           "okteto/test:okteto-cache",
		},
	}
	for _, tt := range tests {
		t.Run(tt.tname, func(t *testing.T) {
			result := GetImageTag(tt.name, tt.namespace, tt.imageTag, tt.deploymentImageTag, tt.oktetoRegistryURL)
			if tt.expected != result {
				t.Errorf("expected %s got %s in test %s", tt.expected, result, tt.tname)
			}
		})
	}
}
