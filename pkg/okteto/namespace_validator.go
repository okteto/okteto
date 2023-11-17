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
	"fmt"
	"regexp"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
)

type namespaceValidator struct {
	nameValidationRegex *regexp.Regexp
	maxAllowedChars     int
}

const (
	previewEnvObject = "preview environment"
)

func newNamespaceValidator() namespaceValidator {
	return namespaceValidator{
		maxAllowedChars:     63,
		nameValidationRegex: regexp.MustCompile("^[a-z0-9]([-a-z0-9]*[a-z0-9])?$"),
	}
}
func (nv namespaceValidator) validate(namespace, object string) error {
	if len(namespace) > nv.maxAllowedChars {
		return oktetoErrors.UserError{
			E: namespaceValidationError{
				object: object,
			},
			Hint: fmt.Sprintf("%s name must be shorter than %d characters.", object, nv.maxAllowedChars),
		}
	}

	if !nv.nameValidationRegex.MatchString(namespace) {
		return oktetoErrors.UserError{
			E: namespaceValidationError{
				object: object,
			},
			Hint: fmt.Sprintf("%s name must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character", object),
		}
	}
	return nil
}
