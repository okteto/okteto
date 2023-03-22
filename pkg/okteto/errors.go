package okteto

import (
	"errors"
	"fmt"
	"time"
)

var (
	// ErrGithubMissingBusinessEmail is raised when the user does not have a business email
	ErrGithubMissingBusinessEmail = errors.New("github-missing-business-email")

	// ErrUnauthorizedGlobalCreation is raised when the user try to create a global preview without permission
	ErrUnauthorizedGlobalCreation = errors.New("you are not authorized to create a global preview env")
)

type pipelineTimeoutError struct {
	pipelineName string
	timeout      time.Duration
}

func (pte pipelineTimeoutError) Error() string {
	return fmt.Sprintf("'%s' didn't finish after %s", pte.pipelineName, pte.timeout.String())
}

type pipelineFailedError struct {
	pipelineName string
}

func (pe pipelineFailedError) Error() string {
	return fmt.Sprintf("pipeline '%s' failed", pe.pipelineName)
}

type previewConflictErr struct {
	name string
}

func (pe previewConflictErr) Error() string {
	return fmt.Sprintf("preview '%s' already exists with a different scope. Please use a different name", pe.name)
}

type namespaceValidationError struct {
	object string
}

func (ne namespaceValidationError) Error() string {
	return fmt.Sprintf("invalid %s name", ne.object)
}

// IsErrGithubMissingBusinessEmail returns true if the error is ErrGithubMissingBusinessEmail
func IsErrGithubMissingBusinessEmail(err error) bool {
	return err.Error() == ErrGithubMissingBusinessEmail.Error()
}

// TranslateAuthError returns a legible error text depending if is custom error
func TranslateAuthError(err error) error {
	switch {
	case IsErrGithubMissingBusinessEmail(err):
		return errors.New("Please add a business/work email to your Github account at https://github.com/settings/emails or try again with a different account")
	default:
		return err
	}
}
