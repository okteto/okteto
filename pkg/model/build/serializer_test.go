// Copyright 2021 The Okteto Authors
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

	yaml "gopkg.in/yaml.v2"
)

func TestImageMashalling(t *testing.T) {
	tests := []struct {
		name     string
		image    Build
		expected string
	}{
		{
			name:     "single-name",
			image:    Build{Name: "image-name"},
			expected: "image-name\n",
		},
		{
			name:     "single-name-and-defaults",
			image:    Build{Name: "image-name", Context: "."},
			expected: "image-name\n",
		},
		{
			name:     "build",
			image:    Build{Name: "image-name", Context: "path"},
			expected: "name: image-name\ncontext: path\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marshalled, err := yaml.Marshal(tt.image)
			if err != nil {
				t.Fatal(err)
			}

			if string(marshalled) != tt.expected {
				t.Errorf("didn't marshal correctly. Actual %s, Expected %s", marshalled, tt.expected)
			}
		})
	}
}
