package suggest

// ErrorSuggestion holds the rules and allows generating suggestions for errors.
type ErrorSuggestion struct {
	rules []Rule
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
