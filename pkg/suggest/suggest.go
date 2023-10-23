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
	rules []ruleInterface
}

// newErrorSuggestion creates a new errorSuggestion instance.
func newErrorSuggestion() *errorSuggestion {
	return &errorSuggestion{}
}

// withRules adds multiple rules to the errorSuggestion.
func (es *errorSuggestion) withRules(rules []ruleInterface) *errorSuggestion {
	for _, r := range rules {
		es.withRule(r)
	}
	return es
}

// withRule adds a new rule to the errorSuggestion.
func (es *errorSuggestion) withRule(rule ruleInterface) *errorSuggestion {
	es.rules = append(es.rules, rule)
	return es
}

// suggest applies all rules and returns a user-friendly error
func (es *errorSuggestion) suggest(err error) error {
	newErr := err
	for _, rule := range es.rules {
		newErr = rule.apply(newErr)
	}
	return newErr
}

type UserFriendlyError struct {
	suggestion *errorSuggestion
	err        error
}

// Error allows UserFriendlyError to satisfy the error interface
func (u *UserFriendlyError) Error() string {
	if err := u.suggestion.suggest(u.err); err != nil {
		return err.Error()
	}
	return ""
}

func NewUserFriendlyError(err error, manifestSchema interface{}) *UserFriendlyError {
	sug := newErrorSuggestion()

	sug.withRules(
		getManifestSuggestionRules(manifestSchema),
	)

	return &UserFriendlyError{
		suggestion: sug,
		err:        err,
	}
}
