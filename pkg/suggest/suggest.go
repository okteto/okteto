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

// errorSuggestion holds the rules and allows generating suggestions for errors.
type errorSuggestion struct {
	rules []*Rule
}

// newErrorSuggestion creates a new errorSuggestion instance.
func newErrorSuggestion() *errorSuggestion {
	return &errorSuggestion{}
}

// WithRules adds multiple rules to the errorSuggestion.
func (es *errorSuggestion) WithRules(rules []*Rule) {
	es.rules = append(es.rules, rules...)
}

// suggest applies all rules and returns a user-friendly error
func (es *errorSuggestion) suggest(err error) error {
	newErr := err
	for _, rule := range es.rules {
		newErr = rule.apply(newErr)
	}
	return newErr
}

// UserFriendlyError is an error that can be used to provide user-friendly error messages
type UserFriendlyError struct {
	suggestion *errorSuggestion
	Err        error
}

// Error allows UserFriendlyError to satisfy the error interface
func (u UserFriendlyError) Error() string {
	if u.Err == nil {
		return ""
	}
	if u.suggestion == nil {
		return u.Err.Error()
	}

	return u.suggestion.suggest(u.Err).Error()
}

func (u UserFriendlyError) Unwrap() error {
	return u.Err
}

func NewUserFriendlyError(err error, rules []*Rule) *UserFriendlyError {
	sug := newErrorSuggestion()

	sug.WithRules(rules)

	return &UserFriendlyError{
		suggestion: sug,
		Err:        err,
	}
}
