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
	"strings"
	"testing"
)

func TestErrorSuggestion(t *testing.T) {
	alwaysFalse := func(e error) bool { return false }
	returnSameErr := func(e error) error { return e }
	emptyRule := NewRule(alwaysFalse, returnSameErr)

	inputError := errors.New("line 4: field contex not found in type model.buildInfoRaw")

	ruleNotFoundInBuildInfoRaw := NewRule(
		func(e error) bool {
			return strings.Contains(e.Error(), "not found in type model.buildInfoRaw")
		},
		func(e error) error {
			return errors.New(strings.Replace(e.Error(), "not found in type model.buildInfoRaw", "does not exist in the build section of the Okteto Manifest", 1))
		},
	)

	tests := []struct {
		name          string
		inputError    error
		rules         []*Rule
		expected      string
		expectedError bool
	}{
		{
			name:       "basic Rule",
			inputError: inputError,
			rules:      []*Rule{ruleNotFoundInBuildInfoRaw},
			expected:   "line 4: field contex does not exist in the build section of the Okteto Manifest",
		},
		{
			name:       "suggesting closest word",
			inputError: inputError,
			rules: []*Rule{
				NewLevenshteinRule(
					"(.*?)field (\\w+) not found in type model.buildInfoRaw",
					"context",
					2,
				),
				ruleNotFoundInBuildInfoRaw,
			},
			expected: "line 4: field contex does not exist in the build section of the Okteto Manifest. Did you mean \"context\"?",
		},
		{
			name:       "no matching Rule",
			inputError: inputError,
			rules: []*Rule{
				emptyRule,
				NewLevenshteinRule(
					"non-matching regex",
					"test",
					2,
				),
			},
			expected: inputError.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errorSuggestion := NewErrorSuggestion()
			errorSuggestion.WithRules(tt.rules)
			suggestion := errorSuggestion.suggest(tt.inputError)

			assert.EqualError(t, suggestion, tt.expected)
		})
	}
}
