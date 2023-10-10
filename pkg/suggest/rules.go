package suggest

import (
	"errors"
	"fmt"
	"github.com/agext/levenshtein"
	"regexp"
	"sigs.k8s.io/kustomize/kyaml/yaml"
	"strings"
)

// NewStrReplaceRule creates a Rule that finds and replaces a string in the error message
func NewStrReplaceRule(find, replace string) Rule {
	condition := func(e error) bool {
		return strings.Contains(e.Error(), find)
	}

	transformation := func(e error) error {
		replacedMessage := strings.ReplaceAll(e.Error(), find, replace)
		return errors.New(replacedMessage)
	}

	return NewRule(condition, transformation)
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

func AddYamlParseErrorHeading() Rule {
	addUrl := func(e error) error {
		errorWithUrlToDocs := fmt.Sprintf("Your okteto manifest is not valid, please check the following errors:\n%s", e.Error())
		return errors.New(errorWithUrlToDocs)
	}

	return NewRule(isYamlError, addUrl)
}

// AddUrlToManifestDocs appends to the error the URL to the Okteto manifest ref docs
func AddUrlToManifestDocs(docsAnchor string) Rule {
	addUrl := func(e error) error {
		docsURL := "https://www.okteto.com/docs/reference/manifest"
		if docsAnchor != "" {
			docsURL += "/#" + docsAnchor
		}
		errorWithUrlToDocs := fmt.Sprintf("%s.\n    Check out the okteto manifest docs at: %s", e.Error(), docsURL)
		return errors.New(errorWithUrlToDocs)
	}

	return NewRule(isYamlError, addUrl)
}

// NewRegexRule creates a Rule based on a regex pattern.
func NewRegexRule(pattern string, transform TransformFunc) Rule {
	re := regexp.MustCompile(pattern)

	condition := func(e error) bool {
		return re.MatchString(e.Error())
	}

	return NewRule(condition, transform)
}

func IndentNumLines() Rule {
	pattern := `(?:yaml: )?line (\d+):`
	re := regexp.MustCompile(pattern)

	condition := func(e error) bool {
		return re.MatchString(e.Error())
	}

	transformation := func(e error) error {
		newErr := re.ReplaceAllString(e.Error(), "   - line $1:")
		return errors.New(newErr)
	}

	return NewRule(condition, transformation)
}

// FieldsNotExistingRule replaces "not found" fields which are unknown to the Okteto manifest specification
func FieldsNotExistingRule() Rule {
	pattern := `field (\w+) not found`
	re := regexp.MustCompile(pattern)

	condition := func(e error) bool {
		return re.MatchString(e.Error())
	}

	transform := func(e error) error {
		newErr := re.ReplaceAllString(e.Error(), "field '$1' is not a property of")
		return fmt.Errorf("%s", newErr)
	}

	return NewRule(condition, transform)
}

// NewLevenshteinRule creates a Rule that matches a regex pattern, extracts a group,
// and computes the Levenshtein distance for that group against a target string.
func NewLevenshteinRule(pattern string, target string) Rule {
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

	return NewRule(condition, transformation)
}

func UserFriendlyError(err error) error {
	yamlErrSuggestion := NewErrorSuggestion()
	yamlErrSuggestion.WithRule(AddYamlParseErrorHeading())

	// TODO: check if we can add the anchor for each section
	yamlErrSuggestion.WithRule(AddUrlToManifestDocs(""))

	keywords := []string{"context", "build", "services", "deploy"}
	//keywords := []string{"build"}
	for _, keyword := range keywords {
		yamlErrSuggestion.WithRule(NewLevenshteinRule(`field (\w+) not found (in type|into) ([\w.]+)`, keyword))
	}

	yamlErrSuggestion.WithRule(FieldsNotExistingRule())

	//Root level
	yamlErrSuggestion.WithRule(NewStrReplaceRule("in type model.manifestRaw", "the okteto manifest"))

	// Build section
	yamlErrSuggestion.WithRule(NewStrReplaceRule("in type model.ManifestBuild", "the 'build' section"))
	yamlErrSuggestion.WithRule(NewStrReplaceRule("into model.ManifestBuild", "into a 'build' object"))
	yamlErrSuggestion.WithRule(NewStrReplaceRule("in type model.buildInfoRaw", "the 'build' object"))

	//YAML data types
	yamlErrSuggestion.WithRule(NewStrReplaceRule(yaml.NodeTagSeq, "list"))
	yamlErrSuggestion.WithRule(NewStrReplaceRule(yaml.NodeTagString, "string"))
	yamlErrSuggestion.WithRule(NewStrReplaceRule(yaml.NodeTagBool, "boolean"))
	yamlErrSuggestion.WithRule(NewStrReplaceRule(yaml.NodeTagInt, "integer"))
	yamlErrSuggestion.WithRule(NewStrReplaceRule(yaml.NodeTagFloat, "float"))
	yamlErrSuggestion.WithRule(NewStrReplaceRule(yaml.NodeTagMap, "object"))

	yamlErrSuggestion.WithRule(NewStrReplaceRule("yaml: unmarshal errors:\n", ""))

	yamlErrSuggestion.WithRule(IndentNumLines())

	return yamlErrSuggestion.Suggest(err)
}
