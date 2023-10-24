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

// ErrorSuggestion holds the rules and allows generating suggestions for errors.
type ErrorSuggestion struct {
	rules []*Rule
}

// UserFriendlyError is an error that can be used to provide user-friendly error messages
type UserFriendlyError struct {
	suggestion *ErrorSuggestion
	err        error
}

// NewErrorSuggestion creates a new ErrorSuggestion instance.
func NewErrorSuggestion() *ErrorSuggestion {
	return &ErrorSuggestion{}
}

// WithRules adds multiple rules to the ErrorSuggestion.
func (es *ErrorSuggestion) WithRules(rules []*Rule) *ErrorSuggestion {
	for _, r := range rules {
		es.rules = append(es.rules, r)
	}
	return es
}

// suggest applies all rules and returns a user-friendly error
func (es *ErrorSuggestion) suggest(err error) error {
	newErr := err
	for _, rule := range es.rules {
		newErr = rule.apply(newErr)
	}
	return newErr
}

// Error allows UserFriendlyError to satisfy the error interface
func (u *UserFriendlyError) Error() string {
	if err := u.suggestion.suggest(u.err); err != nil {
		return err.Error()
	}
	return ""
}

func NewUserFriendlyError(err error, rules []*Rule) *UserFriendlyError {
	sug := NewErrorSuggestion()

	sug.WithRules(rules)

	return &UserFriendlyError{
		suggestion: sug,
		err:        err,
	}
}
