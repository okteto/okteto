// Copyright 2025 The Okteto Authors
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

package waitfor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParser_Parse(t *testing.T) {
	parser := newParser()

	tests := []struct {
		name      string
		input     string
		expectErr error
		result    *parseResult
	}{
		{
			name:      "valid-deployment",
			input:     "deployment/nginx/service_started",
			expectErr: nil,
			result:    &parseResult{"deployment", "nginx", "service_started"},
		},
		{
			name:      "valid-statefulset",
			input:     "statefulset/mysql/service_healthy",
			expectErr: nil,
			result:    &parseResult{"statefulset", "mysql", "service_healthy"},
		},
		{
			name:      "valid-job",
			input:     "job/wake/service_completed_successfully",
			expectErr: nil,
			result:    &parseResult{"job", "wake", "service_completed_successfully"},
		},
		{
			name:      "invalid-format",
			input:     "invalid-format",
			expectErr: errInvalidService,
			result:    nil,
		},
		{
			name:      "invalid-resource",
			input:     "pod/webapp/service_started",
			expectErr: errInvalidResource,
			result:    nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := parser.parse(test.input)
			assert.ErrorIs(t, err, test.expectErr)
			assert.Equal(t, test.result, result)
		})
	}
}
