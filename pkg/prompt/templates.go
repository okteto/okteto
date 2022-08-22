package prompt

import (
	"bytes"
	"fmt"
	"runtime"
	"strings"
	"text/template"

	"github.com/manifoldco/promptui"
)

// oktetoTemplates stores the templates to render the text
type oktetoTemplates struct {
	FuncMap   template.FuncMap
	label     *template.Template
	active    *template.Template
	inactive  *template.Template
	selected  *template.Template
	details   *template.Template
	help      *template.Template
	extraInfo *template.Template
}

func render(tpl *template.Template, data interface{}) []byte {
	var buf bytes.Buffer
	err := tpl.Execute(&buf, data)
	if err != nil {
		return []byte(fmt.Sprintf("%v", data))
	}
	return buf.Bytes()
}

func getSelectedTemplate(selectTpl string) string {
	result := `{{ " ✓ " | bgGreen | black }} {{ .Label | green }}`
	if selectTpl != "" {
		result = fmt.Sprintf(`{{ " ✓ " | bgGreen | black }} {{ "%s '" | green }}{{ .Label | green }}{{ "' selected" | green }}`, selectTpl)
	}

	result = changeColorForWindows(result)
	return result
}

func getActiveTemplate() string {
	whitespaces := ""
	result := fmt.Sprintf("%s%s {{ .Label }}", whitespaces, promptui.IconSelect)
	result = changeColorForWindows(result)
	return result
}

func getInactiveTemplate() string {
	whitespaces := strings.Repeat(" ", 2)
	result := fmt.Sprintf("{{if .Enable}}%s{{ .Label }}{{else}}%s{{ .Label }}{{end}}", whitespaces, whitespaces)
	result = changeColorForWindows(result)
	return result
}

func changeColorForWindows(template string) string {
	if runtime.GOOS == "windows" {
		template = strings.ReplaceAll(template, "oktetoblue", "blue")
	}
	return template
}
