package suggest

import (
	"fmt"
	"github.com/agext/levenshtein"
	"regexp"
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

// NewRegexRule creates a Rule based on a regex pattern.
func NewRegexRule(pattern string, transform TransformFunc) Rule {
	re := regexp.MustCompile(pattern)

	condition := func(e error) bool {
		return re.MatchString(e.Error())
	}

	return NewRule(condition, transform)
}

// NewLevenshteinRule creates a Rule that matches a regex pattern, extracts a group,
// and computes the Levenshtein distance for that group against a target string.
func NewLevenshteinRule(pattern string, target string) Rule {
	re := regexp.MustCompile(pattern)

	condition := func(e error) bool {
		matches := re.FindStringSubmatch(e.Error())
		if len(matches) > 1 {
			distance := levenshtein.Distance(target, matches[1], nil)
			return distance <= 3
		}
		return false
	}

	transformation := func(e error) error {
		return fmt.Errorf("%s. Did you mean \"%s\"?", e.Error(), target)
	}

	return NewRule(condition, transformation)
}

func (g *rule) Translate(err error) error {
	if g.condition(err) {
		return g.transformation(err)
	}
	return err
}
