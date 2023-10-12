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

package suggest

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestUserFriendlyError(t *testing.T) {
	tests := []struct {
		name     string
		input    error
		expected string
	}{
		{
			name:     "empty",
			input:    errors.New(""),
			expected: "",
		},
		{
			name:  "yaml errors have heading",
			input: errors.New("yaml: some random error"),
			expected: `Your okteto manifest is not valid, please check the following errors:
yaml: some random error.
    Check out the okteto manifest docs at: https://www.okteto.com/docs/reference/manifest`,
		},
		{
			name:  "yaml errors have heading",
			input: errors.New("yaml: some random error"),
			expected: `Your okteto manifest is not valid, please check the following errors:
yaml: some random error.
    Check out the okteto manifest docs at: https://www.okteto.com/docs/reference/manifest`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := UserFriendlyError(tt.input)
			assert.Equal(t, tt.expected, err.Error())
		})
	}
}
