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

package manifest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_addKubernetesContext(t *testing.T) {
	var tests = []struct {
		name        string
		dockerfiles []string
		expectedErr bool
	}{
		{
			name:        "only one dockerfile",
			dockerfiles: []string{"Dockerfile"},
			expectedErr: false,
		},
		{
			name:        "different folders",
			dockerfiles: []string{"vote/Dockerfile", "frontend/Dockerfile"},
			expectedErr: false,
		},
		{
			name:        "same folder",
			dockerfiles: []string{"vote/Dockerfile.dev", "vote/Dockerfile.prod"},
			expectedErr: true,
		},
		{
			name:        "same folder on root",
			dockerfiles: []string{"Dockerfile.dev", "Dockerfile.prod"},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDockerfileSelection(tt.dockerfiles)
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
