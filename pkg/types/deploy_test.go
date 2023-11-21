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

package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_DecodeStringToDeployVariable(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []DeployVariable
	}{
		{
			name:     "empty variable",
			input:    "",
			expected: nil,
		},
		{
			name:     "error decoding variables",
			input:    "test",
			expected: nil,
		},
		{
			name:  "success decoding variables",
			input: "W3sibmFtZSI6InRlc3QiLCJ2YWx1ZSI6InZhbHVlIn1d",
			expected: []DeployVariable{
				{
					Name:  "test",
					Value: "value",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := DecodeStringToDeployVariable(tt.input)
			assert.Equal(t, tt.expected, res)
		})
	}
}
