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
	emptyRule := newRule(alwaysFalse, returnSameErr)

	inputError := errors.New("line 4: field contex not found in type model.buildInfoRaw")

	ruleNotFoundInBuildInfoRaw := newRule(
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
		rules         []ruleInterface
		expected      string
		expectedError bool
	}{
		{
			name:       "basic rule",
			inputError: inputError,
			rules:      []ruleInterface{ruleNotFoundInBuildInfoRaw},
			expected:   "line 4: field contex does not exist in the build section of the Okteto Manifest",
		},
		{
			name:       "suggesting closest word",
			inputError: inputError,
			rules: []ruleInterface{
				newLevenshteinRule(
					"field (.+?) not found",
					"context",
				),
				ruleNotFoundInBuildInfoRaw,
			},
			expected: "line 4: field contex does not exist in the build section of the Okteto Manifest. Did you mean \"context\"?",
		},
		{
			name:       "multiple rules and matching regex",
			inputError: inputError,
			rules: []ruleInterface{
				emptyRule,
				emptyRule,
				newRegexRule(`field .+ not found in type model.buildInfoRaw`, func(e error) error {
					return errors.New(strings.Replace(e.Error(), "not found in type model.buildInfoRaw", "does not exist in the build section of the Okteto Manifest", 1))
				}),
				emptyRule,
			},
			expected: "line 4: field contex does not exist in the build section of the Okteto Manifest",
		},
		{
			name:       "no matching rule",
			inputError: inputError,
			rules: []ruleInterface{
				emptyRule,
				newLevenshteinRule(
					"non-matching regex",
					"test",
				),
			},
			expected: inputError.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errorSuggestion := newErrorSuggestion()
			for _, rule := range tt.rules {
				errorSuggestion.withRule(rule)
			}

			suggestion := errorSuggestion.suggest(tt.inputError)

			assert.EqualError(t, suggestion, tt.expected)
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
			expected: `Your okteto manifest is not valid, please check the following errors:
yaml: some random error
    Check out the okteto manifest docs at: https://www.okteto.com/docs/reference/manifest`,
		},
		{
			name:  "yaml errors with heading and link to docs",
			input: errors.New("yaml: unmarshal errors:\n  line 4: field contex not found in type model.manifestRaw"),
			expected: `Your okteto manifest is not valid, please check the following errors:
     - line 4: field 'contex' is not a property of the okteto manifest. Did you mean "context"?
    Check out the okteto manifest docs at: https://www.okteto.com/docs/reference/manifest`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewUserFriendError(tt.input)
			assert.Equal(t, tt.expected, err.Error())
		})
	}
}
