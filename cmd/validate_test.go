// Copyright 2026 The Okteto Authors
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

package cmd

import (
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/require"
)

func Test_validateOktetoManifest_Valid(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
	}{
		{
			name: "deploy commands",
			manifest: `
deploy:
  - kubectl apply -f k8s.yml
  - name: Deploy API
    command: helm upgrade --install api chart`,
		},
		{
			name: "dev sync mode",
			manifest: `
dev:
  api:
    mode: sync
    command: bash`,
		},
		{
			name: "dev hybrid mode",
			manifest: `
dev:
  api:
    mode: hybrid
    command: bash`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, validateOktetoManifest(tt.manifest))
		})
	}
}

func Test_validateOktetoManifest_Invalid(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
		location string
	}{
		{
			name: "mode not in enum",
			manifest: `
dev:
  api:
    mode: unknown`,
			location: "'/dev/api/mode'",
		},
		{
			name: "number instead of string for enum",
			manifest: `
dev:
  api:
    mode: 1`,
			location: "'/dev/api/mode'",
		},
		{
			name: "unknown top level property",
			manifest: `
unknownField: value`,
			location: "at ''",
		},
		{
			name: "invalid second deploy command",
			manifest: `
deploy:
  - kubectl apply -f k8s.yml
  - 5`,
			location: "'/deploy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOktetoManifest(tt.manifest)
			var verr *jsonschema.ValidationError
			require.ErrorAs(t, err, &verr)
			require.ErrorContains(t, err, tt.location)
		})
	}
}
