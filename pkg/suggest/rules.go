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
	"fmt"
	"github.com/agext/levenshtein"
	"regexp"
	"strings"
)

type rule struct {
	condition      conditionFunc
	transformation transformFunc
}

// conditionFunc is a function that returns true if the rule should be applied to the error.
type conditionFunc func(error) bool

// transformFunc is a function that defines how the error should be transformed.
type transformFunc func(error) error

// ruleInterface represents a suggestion rule.
type ruleInterface interface {
	apply(error) error
}

// newRule creates a new ruleInterface instance.
func newRule(condition conditionFunc, transform transformFunc) ruleInterface {
	return &rule{
		condition:      condition,
		transformation: transform,
	}
}

// apply executes the rule on the error.
func (g *rule) apply(err error) error {
	if g.condition(err) {
		return g.transformation(err)
	}
	return err
}

// newStrReplaceRule creates a ruleInterface that finds and replaces a string in the error message
func newStrReplaceRule(find, replace string) ruleInterface {
	condition := func(e error) bool {
		return strings.Contains(e.Error(), find)
	}

	transformation := func(e error) error {
		replacedMessage := strings.ReplaceAll(e.Error(), find, replace)
		return errors.New(replacedMessage)
	}

	return newRule(condition, transformation)
}

// newRegexRule creates a ruleInterface based on a regex pattern.
func newRegexRule(pattern string, transform transformFunc) ruleInterface {
	re := regexp.MustCompile(pattern)

	condition := func(e error) bool {
		return re.MatchString(e.Error())
	}

	return newRule(condition, transform)
}

// newLevenshteinRule creates a ruleInterface that matches a regex pattern, extracts a group,
// and computes the Levenshtein distance for that group against a target string.
func newLevenshteinRule(pattern string, target string) ruleInterface {
	re := regexp.MustCompile("(.*?)" + pattern + "(.*)") // Capture everything before and after the pattern

	condition := func(e error) bool {
		matchingErrors := re.FindAllStringSubmatch(e.Error(), -1)
		for _, matchingError := range matchingErrors {
			distance := levenshtein.Distance(target, matchingError[2], nil)
			if distance <= 3 {
				return true
			}
		}
		return false
	}

	transformation := func(e error) error {
		errorMsg := e.Error()
		matchingErrors := re.FindAllStringSubmatch(e.Error(), -1)

		for _, matchingError := range matchingErrors {
			distance := levenshtein.Distance(target, matchingError[2], nil)
			if distance <= 3 {
				suggestion := fmt.Sprintf("%s. Did you mean \"%s\"?", matchingError[0], target)
				errorMsg = strings.Replace(errorMsg, matchingError[0], suggestion, 1)
			}
		}

		return errors.New(errorMsg)
	}

	return newRule(condition, transformation)
}
