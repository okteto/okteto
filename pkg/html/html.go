package html

import (
	"html/template"
	"net/http"
)

// HTMLTemplateData represents the data merged into the html template
type HTMLTemplateData struct {
	Meta    string
	Origin  string
	Message string
}

func newTemplateData(message string) *HTMLTemplateData {
	return &HTMLTemplateData{
		Meta:    "http-equiv=\"Content-type\" content=\"text/html; charset=utf-8\"",
		Origin:  "the Okteto CLI",
		Message: message,
	}
}

func ExecuteSuccess(w http.ResponseWriter) error {
	data := newTemplateData("Close this window and go back to your terminal")
	successTemplate := template.Must(template.New("success-response").Parse(successTemplateString))
	return successTemplate.Execute(w, data)
}

func ExecuteError(w http.ResponseWriter, err error) error {
	// TODO: check if err is buisness email, render custom page for this error
	data := newTemplateData(err.Error())
	failTemplate := template.Must(template.New("fail-response").Parse(failTemplateString))
	return failTemplate.Execute(w, data)
}
