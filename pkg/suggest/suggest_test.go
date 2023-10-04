package suggest

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestErrorSuggestion(t *testing.T) {
	alwaysFalse := func(e error) bool { return false }
	//alwaysTrue := func(e error) bool { return true }
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
		rules         []Rule
		expected      string
		expectedError bool
	}{
		{
			name:       "basic rule",
			inputError: inputError,
			rules:      []Rule{ruleNotFoundInBuildInfoRaw},
			expected:   "line 4: field contex does not exist in the build section of the Okteto Manifest",
		},
		{
			name:       "suggesting closest word",
			inputError: inputError,
			rules: []Rule{
				NewLevenshteinRule(
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
			rules: []Rule{
				emptyRule,
				emptyRule,
				NewRegexRule(`field .+ not found in type model.buildInfoRaw`, func(e error) error {
					return errors.New(strings.Replace(e.Error(), "not found in type model.buildInfoRaw", "does not exist in the build section of the Okteto Manifest", 1))
				}),
				emptyRule,
			},
			expected: "line 4: field contex does not exist in the build section of the Okteto Manifest",
		},
		{
			name:       "no matching rule",
			inputError: inputError,
			rules: []Rule{
				emptyRule,
				NewLevenshteinRule(
					"non-matching regex",
					"test",
				),
			},
			expected: inputError.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errorSuggestion := NewErrorSuggestion()
			for _, rule := range tt.rules {
				errorSuggestion.WithRule(rule)
			}

			suggestion := errorSuggestion.Suggest(tt.inputError)

			assert.EqualError(t, suggestion, tt.expected)
		})
	}
}
