// Copyright 2022 The Okteto Authors
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

package html

import (
	"html/template"
	"net/http"

	"github.com/okteto/okteto/pkg/errors"
)

type templateData map[string]interface{}

// ExecuteSuccess renders the login success page at the browser
func ExecuteSuccess(w http.ResponseWriter) error {
	data := successData()
	successTemplate := template.Must(template.New("success-response").Parse(commonTemplate))
	return successTemplate.Execute(w, &data)
}

// ExecuteError renders the login error page at the browser
func ExecuteError(w http.ResponseWriter, err error) error {
	var data *templateData

	switch {
	case errors.IsErrGithubMissingBusinessEmail(err):
		data = emailErrorData()
	default:
		data = failData()
	}

	errorTemplate := template.Must(template.New("fail-response").Parse(commonTemplate))
	return errorTemplate.Execute(w, &data)
}
