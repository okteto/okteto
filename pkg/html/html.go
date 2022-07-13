package html

import (
	"html/template"
	"net/http"
)

// TemplateData represents the data merged into the html template
type TemplateData struct {
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
	// TODO: check if err is buisness email, render custom page for this error
	data := failData()
	failTemplate := template.Must(template.New("fail-response").Parse(commonTemplate))
	return failTemplate.Execute(w, &data)
}
