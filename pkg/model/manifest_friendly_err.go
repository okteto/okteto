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

package model

import (
	"errors"
	"fmt"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/suggest"
	"regexp"
	"sigs.k8s.io/kustomize/kyaml/yaml"
	"strings"
)

// newManifestFriendlyError returns a new UserFriendlyError for the okteto manifest.
func newManifestFriendlyError(err error) *suggest.UserFriendlyError {
	rules := getManifestSuggestionRules(Manifest{})
	// we wrap err with oktetoErrors.ErrInvalidManifest because in some parts of the code we check for this error type
	manifestErr := fmt.Errorf("%w:\n%w", oktetoErrors.ErrInvalidManifest, err)
	return suggest.NewUserFriendlyError(manifestErr, rules)
}

// getManifestSuggestionRules returns a collection of rules aiming to improve the error message for the okteto manifest.
func getManifestSuggestionRules(manifestSchema interface{}) []*suggest.Rule {
	rules := []*suggest.Rule{
		addUrlToManifestDocs(),
	}

	// add levenshtein rules to suggest similar field names
	manifestKeys := getStructKeys(manifestSchema)
	manifestKeys["model.manifestRaw"] = manifestKeys["model.Manifest"]
	manifestKeys["model.buildInfoRaw"] = manifestKeys["model.BuildInfo"]
	manifestKeys["model.DevRC"] = manifestKeys["model.Dev"]
	manifestKeys["model.devType"] = manifestKeys["model.Dev"]

	for structName, structKeywords := range manifestKeys {
		for _, keyword := range structKeywords {
			// example: line 5: field contex not found in type model.buildInfoRaw
			// (.*?): this excludes eerything before the keyword "field"
			// (\w+): this captures the keyword we want to calculate the levenshtein distance with
			// (in type|into): this ensures to match all variations of the error message
			// (.*?): this excludes everything after the message that we want to find
			pattern := fmt.Sprintf(`(.*?)field (\w+) not found (in type|into) %s(.*?)`, structName)

			// keywordInGroup is the index of the capturing group that contains the actual mistyped keyword
			// set to 2 because index 0 is the whole sentence and index 1 is "line 5"
			keywordInGroup := 2
			rule := suggest.NewLevenshteinRule(pattern, keyword, keywordInGroup)
			rules = append(rules, rule)
		}
	}

	rules = append(rules,
		// invalid properties
		fieldsNotExistingRule(),

		// struct names
		suggest.NewStrReplaceRule("in type model.manifestRaw", "the okteto manifest"),
		suggest.NewStrReplaceRule("in type model.ManifestBuild", "the 'build' section"),
		suggest.NewStrReplaceRule("into model.ManifestBuild", "into a 'build' object"),
		suggest.NewStrReplaceRule("in type model.buildInfoRaw", "the 'build' object"),
		suggest.NewStrReplaceRule("in type model.devType", "the 'dev' object"),
		suggest.NewStrReplaceRule("into model.devType", "the 'dev' object"),

		// yaml types
		suggest.NewStrReplaceRule(yaml.NodeTagSeq, "list"),
		suggest.NewStrReplaceRule(yaml.NodeTagString, "string"),
		suggest.NewStrReplaceRule(yaml.NodeTagBool, "boolean"),
		suggest.NewStrReplaceRule(yaml.NodeTagInt, "integer"),
		suggest.NewStrReplaceRule(yaml.NodeTagFloat, "float"),
		suggest.NewStrReplaceRule(yaml.NodeTagMap, "object"),

		// misc
		suggest.NewStrReplaceRule("yaml: unmarshal errors:\n", ""),
		indentNumLines(),
	)

	return rules
}

func isYamlErrorWithoutLinkToDocs(err error) bool {
	return isYamlError(err) && !hasLinkToDocs(err)
}

// hasLinkToDocs checks if the error already contains a link to our docs
func hasLinkToDocs(err error) bool {
	return strings.Contains(err.Error(), "https://www.okteto.com/docs")
}

// isYamlError check if the error is related to YAML unmarshalling
func isYamlError(err error) bool {
	if err == nil {
		return false
	}
	// detect yaml errors by prefix
	if strings.Contains(err.Error(), "yaml:") {
		return true
	}
	// detect yaml errors by type
	var typeError *yaml.TypeError
	return errors.As(err, &typeError)
}

// addUrlToManifestDocs appends to the error the URL to the Okteto manifest ref docs
func addUrlToManifestDocs() *suggest.Rule {
	addUrl := func(e error) error {
		docsURL := "https://www.okteto.com/docs/reference/manifest"
		errorWithUrlToDocs := fmt.Sprintf("%s\n    Check out the okteto manifest docs at: %s", e.Error(), docsURL)
		return errors.New(errorWithUrlToDocs)
	}

	return suggest.NewRule(isYamlErrorWithoutLinkToDocs, addUrl)
}

func indentNumLines() *suggest.Rule {
	pattern := `(?:yaml: )?line (\d+):`
	re := regexp.MustCompile(pattern)

	condition := func(e error) bool {
		return re.MatchString(e.Error())
	}

	transformation := func(e error) error {
		newErr := re.ReplaceAllString(e.Error(), "   - line $1:")
		return errors.New(newErr)
	}

	return suggest.NewRule(condition, transformation)
}

// fieldsNotExistingRule replaces "not found" fields which are unknown to the Okteto manifest specification
func fieldsNotExistingRule() *suggest.Rule {
	pattern := `field (\w+) not found`
	re := regexp.MustCompile(pattern)

	condition := func(e error) bool {
		return re.MatchString(e.Error())
	}

	transform := func(e error) error {
		newErr := re.ReplaceAllString(e.Error(), "field '$1' is not a property of")
		return fmt.Errorf("%s", newErr)
	}

	return suggest.NewRule(condition, transform)
}
