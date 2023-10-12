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
	"github.com/okteto/okteto/pkg/model"
	"reflect"
	"regexp"
	"sigs.k8s.io/kustomize/kyaml/yaml"
	"strings"
)

// getManifestSuggestionRules returns a collection of rules aiming to improve the error message for the okteto manifest.
func getManifestSuggestionRules() []ruleInterface {
	rules := []ruleInterface{
		addYamlParseErrorHeading(),
		addUrlToManifestDocs(""),
	}

	// add levenshtein rules to suggest similar field names
	manifestKeys := getStructKeys(model.Manifest{})
	if manifestKeys["model.Manifest"] != nil {
		manifestKeys["model.manifestRaw"] = manifestKeys["model.Manifest"]
	}
	for structName, structKeywords := range manifestKeys {
		for _, keyword := range structKeywords {
			rule := newLevenshteinRule(fmt.Sprintf(`field (\w+) not found (in type|into) %s`, structName), keyword)
			rules = append(rules, rule)
		}
	}

	// add fieldsNotExistingRule()
	rules = append(rules,
		// invalid properties
		fieldsNotExistingRule(),

		// struct names
		newStrReplaceRule("in type model.manifestRaw", "the okteto manifest"),
		newStrReplaceRule("in type model.ManifestBuild", "the 'build' section"),
		newStrReplaceRule("into model.ManifestBuild", "into a 'build' object"),
		newStrReplaceRule("in type model.buildInfoRaw", "the 'build' object"),

		// yaml types
		newStrReplaceRule(yaml.NodeTagSeq, "list"),
		newStrReplaceRule(yaml.NodeTagString, "string"),
		newStrReplaceRule(yaml.NodeTagBool, "boolean"),
		newStrReplaceRule(yaml.NodeTagInt, "integer"),
		newStrReplaceRule(yaml.NodeTagFloat, "float"),
		newStrReplaceRule(yaml.NodeTagMap, "object"),

		// misc
		newStrReplaceRule("yaml: unmarshal errors:\n", ""),
		indentNumLines(),
	)

	return rules
}

func isYamlError(err error) bool {
	// prevents adding the URL twice
	if strings.Contains(err.Error(), "https://www.okteto.com/docs") {
		return false
	}
	// only add the URL if it's a yaml error
	if _, ok := err.(*yaml.TypeError); ok {
		return true
	}
	if strings.Contains(err.Error(), "yaml:") {
		return true
	}

	return false
}

func addYamlParseErrorHeading() ruleInterface {
	addUrl := func(e error) error {
		errorWithUrlToDocs := fmt.Sprintf("Your okteto manifest is not valid, please check the following errors:\n%s", e.Error())
		return errors.New(errorWithUrlToDocs)
	}

	return newRule(isYamlError, addUrl)
}

// addUrlToManifestDocs appends to the error the URL to the Okteto manifest ref docs
func addUrlToManifestDocs(docsAnchor string) ruleInterface {
	addUrl := func(e error) error {
		docsURL := "https://www.okteto.com/docs/reference/manifest"
		if docsAnchor != "" {
			docsURL += "/#" + docsAnchor
		}
		errorWithUrlToDocs := fmt.Sprintf("%s.\n    Check out the okteto manifest docs at: %s", e.Error(), docsURL)
		return errors.New(errorWithUrlToDocs)
	}

	return newRule(isYamlError, addUrl)
}

func indentNumLines() ruleInterface {
	pattern := `(?:yaml: )?line (\d+):`
	re := regexp.MustCompile(pattern)

	condition := func(e error) bool {
		return re.MatchString(e.Error())
	}

	transformation := func(e error) error {
		newErr := re.ReplaceAllString(e.Error(), "   - line $1:")
		return errors.New(newErr)
	}

	return newRule(condition, transformation)
}

// fieldsNotExistingRule replaces "not found" fields which are unknown to the Okteto manifest specification
func fieldsNotExistingRule() ruleInterface {
	pattern := `field (\w+) not found`
	re := regexp.MustCompile(pattern)

	condition := func(e error) bool {
		return re.MatchString(e.Error())
	}

	transform := func(e error) error {
		newErr := re.ReplaceAllString(e.Error(), "field '$1' is not a property of")
		return fmt.Errorf("%s", newErr)
	}

	return newRule(condition, transform)
}

func appendUnique(slice1 []string, slice2 []string) []string {
	for _, val := range slice2 {
		unique := true
		for _, v := range slice1 {
			if v == val {
				unique = false
				break
			}
		}
		if unique {
			slice1 = append(slice1, val)
		}
	}
	return slice1
}

// getStructKeys recursively goes through a given struct and returns a map of struct names to their fields
func getStructKeys(t interface{}) map[string][]string {
	result := make(map[string][]string)
	typ := reflect.TypeOf(t)

	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	// Handle map type at top level
	if typ.Kind() == reflect.Map {
		// For each key in the map type, check if the value type is a struct and process accordingly
		mapValueType := typ.Elem()
		if mapValueType.Kind() == reflect.Ptr {
			mapValueType = mapValueType.Elem()
		}

		if mapValueType.Kind() == reflect.Struct {
			return getStructKeys(reflect.New(mapValueType).Interface())
		}
		return result
	}

	if typ.Kind() != reflect.Struct {
		return result
	}

	var structFullName string

	if typ.Name() == "" { // Anonymous struct
		structFullName = "_"
	} else {
		pkgPathSegments := strings.Split(typ.PkgPath(), "/")
		packageName := pkgPathSegments[len(pkgPathSegments)-1]
		structFullName = packageName + "." + typ.Name()
	}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		fieldType := field.Type

		if fieldType.Kind() == reflect.Struct {
			for k, v := range getStructKeys(reflect.New(fieldType).Interface()) {
				result[k] = appendUnique(result[k], v)
			}
		} else if fieldType.Kind() == reflect.Map {
			if fieldType.Key().Kind() == reflect.String {
				yamlTag := field.Tag.Get("yaml")
				if yamlTag != "" && yamlTag != "-" {
					parts := strings.Split(yamlTag, ",")
					if len(parts) > 0 {
						result[structFullName] = append(result[structFullName], parts[0])
					}
				}
			}
			// Recurse if the value type of the map is a pointer-to-struct
			mapValueType := fieldType.Elem()
			if mapValueType.Kind() == reflect.Ptr && mapValueType.Elem().Kind() == reflect.Struct {
				for k, v := range getStructKeys(reflect.New(mapValueType.Elem()).Interface()) {
					result[k] = appendUnique(result[k], v)
				}
			}
		} else if fieldType.Kind() == reflect.Ptr && fieldType.Elem().Kind() == reflect.Struct {
			for k, v := range getStructKeys(reflect.New(fieldType.Elem()).Interface()) {
				result[k] = appendUnique(result[k], v)
			}
		} else if fieldType.Kind() == reflect.Slice && fieldType.Elem().Kind() == reflect.Struct {
			for k, v := range getStructKeys(reflect.New(fieldType.Elem()).Interface()) {
				result[k] = appendUnique(result[k], v)
			}
		} else {
			yamlTag := field.Tag.Get("yaml")
			if yamlTag != "" && yamlTag != "-" {
				parts := strings.Split(yamlTag, ",")
				if len(parts) > 0 {
					result[structFullName] = append(result[structFullName], parts[0])
				}
			}
		}
	}

	return result
}
