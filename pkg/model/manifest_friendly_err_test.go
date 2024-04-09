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

package model

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_isYamlErrorWithoutLinkToDocs(t *testing.T) {
	tests := []struct {
		input    error
		name     string
		expected bool
	}{
		{
			name:     "random error",
			input:    errors.New("random error"),
			expected: false,
		},
		{
			name:     "nil",
			input:    nil,
			expected: false,
		},
		{
			name:     "yaml error with link to docs",
			input:    errors.New("yaml: some random error. See https://www.okteto.com/docs"),
			expected: false,
		},
		{
			name:     "yaml error without link to docs",
			input:    errors.New("yaml: some random error"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := isYamlErrorWithoutLinkToDocs(tt.input)
			assert.Equal(t, tt.expected, res)
		})
	}
}

func TestUserFriendlyError(t *testing.T) {
	tests := []struct {
		name     string
		input    error
		expected string
	}{
		{
			name:  "yaml errors with heading and link to docs",
			input: errors.New("yaml: some random error"),
			expected: `your okteto manifest is not valid, please check the following errors:
yaml: some random error
    Check out the okteto manifest docs at: https://www.okteto.com/docs/reference/okteto-manifest`,
		},
		{
			name:  "yaml errors with heading and link to docs",
			input: errors.New("yaml: unmarshal errors:\n  line 4: field kontext not found in type model.manifestRaw"),
			expected: `your okteto manifest is not valid, please check the following errors:
     - line 4: field 'kontext' is not a property of the okteto manifest. Did you mean "context"?
    Check out the okteto manifest docs at: https://www.okteto.com/docs/reference/okteto-manifest`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := newManifestFriendlyError(tt.input)
			assert.Equal(t, tt.expected, err.Error())
		})
	}
}
