package suggest

import (
	"errors"
	"fmt"
	"github.com/agext/levenshtein"
	"gopkg.in/yaml.v3"
	"regexp"
	"strings"
)

type ConditionFunc func(error) bool
type TransformFunc func(error) error

type Rule interface {
	Translate(error) error
}

type rule struct {
	condition      ConditionFunc
	transformation TransformFunc
}

func NewRule(condition ConditionFunc, transform TransformFunc) Rule {
	return &rule{
		condition:      condition,
		transformation: transform,
	}
}

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

//func ConvertTypeError() Rule {
//	cannot unmarshal !!str `todo-list` into model.ManifestBuild
//}

//func MakeStructNamesReadable() Rule {
//	NewStrReplaceRule
//}

//func MapYamlTypes() Rule {
//
//}

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
		errorWithUrlToDocs := fmt.Sprintf("%s.\nCheck out the okteto manifest docs at: %s", e.Error(), docsURL)
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
//func _NewLevenshteinRule(pattern string, target string) Rule {
//	re := regexp.MustCompile(pattern)
//
//	condition := func(e error) bool {
//		matches := re.FindStringSubmatch(e.Error())
//		if len(matches) > 1 {
//			distance := levenshtein.Distance(target, matches[1], nil)
//			return distance <= 3
//		}
//		return false
//	}
//
//	transformation := func(e error) error {
//		return fmt.Errorf("%s. Did you mean \"%s\"?", e.Error(), target)
//	}
//
//	return NewRule(condition, transformation)
//}

// NewLevenshteinRule creates a Rule that matches a regex pattern, extracts a group,
// and computes the Levenshtein distance for that group against a target string.
func NewLevenshteinRule(pattern string, target string) Rule {
	re := regexp.MustCompile("(.*?)" + pattern + "(.*)") // Capture everything before and after the pattern

	condition := func(e error) bool {
		matches := re.FindStringSubmatch(e.Error())
		if len(matches) > 3 { // matches[0] is the entire match, matches[1] and matches[3] capture before and after the pattern
			distance := levenshtein.Distance(target, matches[2], nil)
			return distance <= 3
		}
		return false
	}

	transformation := func(e error) error {
		errorMsg := e.Error()
		matches := re.FindAllStringSubmatch(errorMsg, -1)

		for _, match := range matches {
			if len(match) > 3 {
				word := match[2]
				distance := levenshtein.Distance(word, target, nil)

				if distance <= 3 {
					old := fmt.Sprintf("field %s not found %s %s", word, match[3], match[4])
					replacement := fmt.Sprintf("%s. Did you mean \"%s\"?", old, target)
					errorMsg = strings.Replace(errorMsg, old, replacement, 1)
				}
			}
		}

		return errors.New(errorMsg)
	}

	return NewRule(condition, transformation)
}

func (g *rule) Translate(err error) error {
	if g.condition(err) {
		return g.transformation(err)
	}
	return err
}
