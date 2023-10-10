package suggest

// ErrorSuggestion holds the rules and allows generating suggestions for errors.
type ErrorSuggestion struct {
	rules []Rule
}

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

func (g *rule) Translate(err error) error {
	if g.condition(err) {
		return g.transformation(err)
	}
	return err
}

// NewErrorSuggestion creates a new ErrorSuggestion instance.
func NewErrorSuggestion() *ErrorSuggestion {
	return &ErrorSuggestion{}
}

// WithRule adds a new rule to the ErrorSuggestion.
func (es *ErrorSuggestion) WithRule(rule Rule) *ErrorSuggestion {
	es.rules = append(es.rules, rule)
	return es
}

// Suggest generates a user-friendly suggestion for the given error.
func (es *ErrorSuggestion) Suggest(err error) error {
	newErr := err
	for _, rule := range es.rules {
		newErr = rule.Translate(newErr)
	}
	return newErr
}
