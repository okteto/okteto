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

func Test_NewStrReplaceRule(t *testing.T) {
	inputErr := errors.New("a random test error: replace-me")
	r := NewStrReplaceRule("replace-me", "new-value")
	err := r.apply(inputErr)
	assert.Equal(t, "a random test error: new-value", err.Error())
}

func Test_NewLevenshteinRule(t *testing.T) {
	type ruleInput struct {
		pattern            string
		target             string
		targetInGroupIndex int
	}

	tests := []struct {
		name      string
		ruleInput ruleInput
		inputErr  error
		expected  string
	}{
		{
			name:     "no match",
			inputErr: assert.AnError,
			ruleInput: ruleInput{
				pattern:            "some-pattern-not-found",
				target:             "some-target-keyword",
				targetInGroupIndex: 0,
			},
			expected: assert.AnError.Error(),
		},
		{
			name:     "match but wrong index",
			inputErr: assert.AnError,
			ruleInput: ruleInput{
				pattern:            "(.*)",
				target:             "some-target-keyword",
				targetInGroupIndex: 10,
			},
			expected: assert.AnError.Error(),
		},
		{
			name:     "match with suggestion",
			inputErr: errors.New("an error occurred: unix-tests"),
			ruleInput: ruleInput{
				pattern:            "an error occurred: (.*)",
				target:             "unit-tests",
				targetInGroupIndex: 1,
			},
			expected: "an error occurred: unix-tests. Did you mean \"unit-tests\"?",
		},
		{
			name:     "match with suggestion and short target",
			inputErr: errors.New("an error occurred: ui"),
			ruleInput: ruleInput{
				pattern:            "an error occurred: (.*)",
				target:             "ux",
				targetInGroupIndex: 1,
			},
			expected: "an error occurred: ui. Did you mean \"ux\"?",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewLevenshteinRule(tt.ruleInput.pattern, tt.ruleInput.target, tt.ruleInput.targetInGroupIndex)
			err := r.apply(tt.inputErr)
			assert.Equal(t, tt.expected, err.Error())
		})
	}
}

func TestNewLevenshteinRule_ErrWithoutPanic(t *testing.T) {
	inputErr := errors.New("a random test error")
	r := NewLevenshteinRule("(", "test", 0)
	err := r.apply(inputErr)

	assert.Equal(t, "a random test error", err.Error())
}
