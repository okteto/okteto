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
