package suggest

import "regexp"

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

func (g *rule) Translate(err error) error {
	if g.condition(err) {
		return g.transformation(err)
	}
	return err
}
