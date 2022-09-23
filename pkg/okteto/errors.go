package okteto

import "errors"

var (
	//ErrGithubMissingBusinessEmail is raised when the user does not have a business email
	ErrGithubMissingBusinessEmail = errors.New("github-missing-business-email")
)

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
