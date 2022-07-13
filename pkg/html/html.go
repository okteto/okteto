package html

import (
	"html/template"
	"net/http"

	"github.com/okteto/okteto/pkg/errors"
)

type templateData struct {
	Icon    template.HTML
	Title   string
	Content template.HTML
}

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
