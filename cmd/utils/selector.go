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

package utils

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"text/template"

	"github.com/chzyer/readline"
	"github.com/juju/ansiterm"
	"github.com/manifoldco/promptui"
	"github.com/manifoldco/promptui/list"
	"github.com/manifoldco/promptui/screenbuf"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
)

const (
	esc        = "\033["
	hideCursor = esc + "?25l"
	showCursor = esc + "?25h"
	clearLine  = esc + "2K"
)

// OktetoSelector represents the selector
type OktetoSelector struct {
	Label string
	Items []SelectorItem
	Size  int

	Templates *promptui.SelectTemplates
	Keys      *promptui.SelectKeys

	OktetoTemplates *oktetoTemplates
}

//oktetoTemplates stores the templates to render the text
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

//SelectorItem represents a selectable item on a selector
type SelectorItem struct {
	Name      string
	Label     string
	Enable    bool
	IsOkteto  bool
	Namespace string
	Builder   string
	Registry  string
}

//AskForOptionsOkteto given some options ask the user to select one
func AskForOptionsOkteto(ctx context.Context, options []SelectorItem, label string) (string, bool, error) {
	selectedTemplate := getSelectedTemplate()
	activeTemplate := getActiveTemplate()
	inactiveTemplate := getInactiveTemplate()

	prompt := OktetoSelector{
		Label: label,
		Items: options,
		Size:  len(options),
		Templates: &promptui.SelectTemplates{
			Label:    "{{ .Label }}",
			Selected: selectedTemplate,
			Active:   activeTemplate,
			Inactive: inactiveTemplate,
			FuncMap:  promptui.FuncMap,
		},
	}

	prompt.Templates.FuncMap["oktetoblue"] = oktetoLog.BlueString
	optionSelected, isOkteto, err := prompt.Run(ctx)
	if err != nil || !isValidOption(options, optionSelected) {
		oktetoLog.Infof("invalid init option: %s", err)
		return "", false, fmt.Errorf("invalid option")
	}

	return optionSelected, isOkteto, nil
}

func isValidOption(options []SelectorItem, optionSelected string) bool {
	for _, option := range options {
		if optionSelected == option.Name {
			return true
		}
	}
	return false
}

//Run runs the selector prompt
func (s *OktetoSelector) Run(ctx context.Context) (string, bool, error) {
	startPosition, err := s.getInitialPosition(ctx)
	if err != nil {
		return "", false, err
	}
	if startPosition != -1 {
		s.Items[startPosition].Label += " *"
	}
	l, err := list.New(s.Items, s.Size)
	if err != nil {
		return "", false, err
	}

	s.prepareTemplates()

	s.Keys = &promptui.SelectKeys{
		Prev:     promptui.Key{Code: promptui.KeyPrev, Display: promptui.KeyPrevDisplay},
		Next:     promptui.Key{Code: promptui.KeyNext, Display: promptui.KeyNextDisplay},
		PageUp:   promptui.Key{Code: promptui.KeyBackward, Display: promptui.KeyBackwardDisplay},
		PageDown: promptui.Key{Code: promptui.KeyForward, Display: promptui.KeyForwardDisplay},
	}
	c := &readline.Config{}
	err = c.Init()
	if err != nil {
		return "", false, err
	}
	if runtime.GOOS != "windows" {
		c.Stdout = &stdout{}
	}

	c.Stdin = readline.NewCancelableStdin(c.Stdin)

	c.HistoryLimit = -1
	c.UniqueEditLine = true

	rl, err := readline.NewEx(c)
	if err != nil {
		return "", false, err
	}

	sb := screenbuf.New(rl)
	l.SetCursor(startPosition)

	c.SetListener(func(line []rune, pos int, key rune) ([]rune, int, bool) {
		switch {
		case key == promptui.KeyEnter:
			return nil, 0, true
		case key == s.Keys.Next.Code:
			nextItemIndex := l.Index() + 1
			if s.Size > nextItemIndex && !s.Items[nextItemIndex].Enable {
				l.Next()
			}
			l.Next()
		case key == s.Keys.Prev.Code:
			currentIdx := l.Index()
			prevItemIndex := currentIdx - 1
			foundNewActive := false
			for prevItemIndex > -1 {
				l.Prev()
				if !s.Items[prevItemIndex].Enable {
					prevItemIndex--
					continue
				}
				foundNewActive = true
				break
			}
			if !foundNewActive {
				l.SetCursor(currentIdx)
			}

		case key == s.Keys.PageUp.Code:
			l.PageUp()
		case key == s.Keys.PageDown.Code:
			l.PageDown()
		}

		s.renderLabel(sb)

		help := s.renderHelp()
		sb.Write(help)

		items, idx := l.Items()
		last := len(items) - 1

		for i, item := range items {
			page := " "

			switch i {
			case 0:
				if l.CanPageUp() {
					page = "↑"
				} else {
					page = string(' ')
				}
			case last:
				if l.CanPageDown() {
					page = "↓"
				}
			}

			output := []byte(page + " ")

			if i == idx {
				output = append(output, render(s.OktetoTemplates.active, item)...)
			} else {
				output = append(output, render(s.OktetoTemplates.inactive, item)...)
			}

			sb.Write(output)
		}

		if idx == list.NotFound {
			sb.WriteString("")
			sb.WriteString("No results")
		} else {
			active := items[idx]

			details := s.renderDetails(active)
			for _, d := range details {
				sb.Write(d)
			}
		}

		sb.Flush()

		return nil, 0, true
	})
	for {
		_, err = rl.Readline()

		if err != nil {
			switch {
			case err == readline.ErrInterrupt, err.Error() == "Interrupt":
				err = promptui.ErrInterrupt
			case err == io.EOF:
				err = promptui.ErrEOF
			}
			break
		}

		_, idx := l.Items()
		if idx != list.NotFound {
			break
		}

	}

	if err != nil {
		if err.Error() == "Interrupt" {
			err = promptui.ErrInterrupt
		}
		sb.Reset()
		sb.WriteString("")
		sb.Flush()
		rl.Write([]byte(showCursor))
		rl.Close()
		return "", false, err
	}

	items, idx := l.Items()
	item := items[idx]

	sb.Reset()
	sb.Write(render(s.OktetoTemplates.selected, item))
	sb.Flush()

	rl.Write([]byte(showCursor))
	rl.Close()

	return s.Items[l.Index()].Name, s.Items[l.Index()].IsOkteto, err
}

func (s *OktetoSelector) prepareTemplates() error {
	tpls := s.OktetoTemplates
	if tpls == nil {
		tpls = &oktetoTemplates{}
	}

	tpls.FuncMap = s.Templates.FuncMap

	if s.Templates.Label == "" {
		s.Templates.Label = fmt.Sprintf("%s {{.}}: ", promptui.IconInitial)
	}

	tpl, err := template.New("").Funcs(tpls.FuncMap).Parse(s.Templates.Label)
	if err != nil {
		return err
	}

	tpls.label = tpl

	if s.Templates.Active == "" {
		s.Templates.Active = fmt.Sprintf("%s {{ . | underline }}", promptui.IconSelect)
	}

	tpl, err = template.New("").Funcs(tpls.FuncMap).Parse(s.Templates.Active)
	if err != nil {
		return err
	}

	tpls.active = tpl

	if s.Templates.Inactive == "" {
		s.Templates.Inactive = "  {{.}}"
	}

	tpl, err = template.New("").Funcs(tpls.FuncMap).Parse(s.Templates.Inactive)
	if err != nil {
		return err
	}

	tpls.inactive = tpl

	if s.Templates.Selected == "" {
		s.Templates.Selected = fmt.Sprintf(`{{ "%s" | green }} {{ . | faint }}`, promptui.IconGood)
	}

	tpl, err = template.New("").Funcs(tpls.FuncMap).Parse(s.Templates.Selected)
	if err != nil {
		return err
	}
	tpls.selected = tpl

	if s.Templates.Details != "" {
		tpl, err = template.New("").Funcs(tpls.FuncMap).Parse(s.Templates.Details)
		if err != nil {
			return err
		}

		tpls.details = tpl
	}

	if s.Templates.Help == "" {
		s.Templates.Help = fmt.Sprintf(`{{ "Use the arrow keys to navigate:" | faint }} {{ .NextKey | faint }} ` +
			`{{ .PrevKey | faint }} {{ .PageDownKey | faint }} {{ .PageUpKey | faint }} ` +
			`{{ if .Search }} {{ "and" | faint }} {{ .SearchKey | faint }} {{ "toggles search" | faint }}{{ end }}`)
	}

	tpl, err = template.New("").Funcs(tpls.FuncMap).Parse(s.Templates.Help)
	if err != nil {
		return err
	}

	tpls.help = tpl

	extraInfo := changeColorForWindows(`{{ " i " | black | bgBlue }} {{ "Use 'okteto context <URL>' to add a new cluster context" | oktetoblue }}`)

	tpl, err = template.New("").Funcs(tpls.FuncMap).Parse(extraInfo)
	if err != nil {
		return err
	}

	tpls.extraInfo = tpl

	s.OktetoTemplates = tpls

	return nil
}

func (s OktetoSelector) getInitialPosition(ctx context.Context) (int, error) {
	ctxStore := okteto.ContextStore()
	oCtx := ctxStore.CurrentContext
	if oCtx == "" {
		return -1, nil
	}

	oCtx = okteto.K8sContextToOktetoUrl(ctx, oCtx, ctxStore.Contexts[oCtx].Namespace, okteto.NewK8sClientProvider())
	idx := 0
	for _, item := range s.Items {
		if strings.Contains(item.Name, oCtx) {
			return idx, nil
		}
		idx++
	}
	return -1, nil
}

func (s *OktetoSelector) renderDetails(item interface{}) [][]byte {
	if s.OktetoTemplates.details == nil {
		return nil
	}

	var buf bytes.Buffer
	w := ansiterm.NewTabWriter(&buf, 0, 0, 8, ' ', 0)

	err := s.OktetoTemplates.details.Execute(w, item)
	if err != nil {
		fmt.Fprintf(w, "%v", item)
	}

	w.Flush()

	output := buf.Bytes()

	return bytes.Split(output, []byte("\n"))
}

func (s *OktetoSelector) renderLabel(sb *screenbuf.ScreenBuf) {
	for _, labelLine := range strings.Split(s.Label, "\n") {
		labelLineBytes := render(s.OktetoTemplates.label, labelLine)
		sb.Write(labelLineBytes)
	}
}

func (s *OktetoSelector) renderHelp() []byte {
	keys := struct {
		NextKey     string
		PrevKey     string
		PageDownKey string
		PageUpKey   string
		Search      bool
		SearchKey   string
	}{
		NextKey:     s.Keys.Next.Display,
		PrevKey:     s.Keys.Prev.Display,
		PageDownKey: s.Keys.PageDown.Display,
		PageUpKey:   s.Keys.PageUp.Display,
		SearchKey:   s.Keys.Search.Display,
	}

	return render(s.OktetoTemplates.help, keys)
}

func render(tpl *template.Template, data interface{}) []byte {
	var buf bytes.Buffer
	err := tpl.Execute(&buf, data)
	if err != nil {
		return []byte(fmt.Sprintf("%v", data))
	}
	return buf.Bytes()
}

type stdout struct{}

// Write implements an io.WriterCloser over os.Stderr, but it skips the terminal
// bell character.
func (*stdout) Write(b []byte) (int, error) {
	if len(b) == 1 && b[0] == readline.CharBell {
		return 0, nil
	}
	return os.Stderr.Write(b)
}

// Close implements an io.WriterCloser over os.Stderr.
func (*stdout) Close() error {
	return os.Stderr.Close()
}

func getSelectedTemplate() string {
	result := `{{ " ✓ " | bgGreen | black }} {{ .Label | green }}`
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
