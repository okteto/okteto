// Copyright 2024 The Okteto Authors
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

package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Icon(t *testing.T) {
	tests := []struct {
		name      string
		manifest  string
		expectErr bool
	}{
		{
			name: "empty",
			manifest: `
icon: `,
		},
		{
			name: "valid enum value",
			manifest: `
icon: database`,
		},
		{
			name: "valid custom value",
			manifest: `
icon: my-custom-icon`,
		},
		{
			name: "invalid type",
			manifest: `
icon: 
  name: invalid`,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOktetoManifest(tt.manifest)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
