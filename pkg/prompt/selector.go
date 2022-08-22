package prompt

import (
	"bytes"
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
	"golang.org/x/term"
)

const (
	esc        = "\033["
	showCursor = esc + "?25h"
)

type OktetoSelectorInterface interface {
	Ask() (string, error)
}

// OktetoSelector represents the selector
type OktetoSelector struct {
	Label string
	items []selectorItem
	size  int

	templates *promptui.SelectTemplates
	keys      *promptui.SelectKeys

	oktetoTemplates *oktetoTemplates
	initialPosition int
}

// selectorItem represents a selectable item on a selector
type selectorItem struct {
	Name   string
	Label  string
	Enable bool
}

func NewOktetoSelector(label string, options []string, selectedTpl string, preselected int) *OktetoSelector {
	items := optionsToSelectorItem(options)
	selectedTemplate := getSelectedTemplate(selectedTpl)
	activeTemplate := getActiveTemplate()
	inactiveTemplate := getInactiveTemplate()

	selector := &OktetoSelector{
		Label: label,
		items: items,
		size:  len(items),
		templates: &promptui.SelectTemplates{
			Label:    "{{ .Label }}",
			Selected: selectedTemplate,
			Active:   activeTemplate,
			Inactive: inactiveTemplate,
			FuncMap:  promptui.FuncMap,
		},
		initialPosition: preselected,
	}

	if preselected != -1 {
		selector.initialPosition = preselected
		selector.items[preselected].Label += " *"
	}
	return selector
}

// Ask given some options ask the user to select one
func (s *OktetoSelector) Ask() (string, error) {
	s.templates.FuncMap["oktetoblue"] = oktetoLog.BlueString
	optionSelected, err := s.run()
	if err != nil || !isValidOption(s.items, optionSelected) {
		oktetoLog.Infof("invalid init option: %s", err)
		return "", ErrInvalidOption
	}

	return optionSelected, nil
}

func isValidOption(options []selectorItem, optionSelected string) bool {
	for _, option := range options {
		if optionSelected == option.Name {
			return true
		}
	}
	return false
}

// run runs the selector prompt
func (s *OktetoSelector) run() (string, error) {
	l, err := list.New(s.items, s.size)
	if err != nil {
		return "", err
	}
	l.SetCursor(s.initialPosition)

	s.prepareTemplates()

	s.keys = &promptui.SelectKeys{
		Prev:     promptui.Key{Code: promptui.KeyPrev, Display: promptui.KeyPrevDisplay},
		Next:     promptui.Key{Code: promptui.KeyNext, Display: promptui.KeyNextDisplay},
		PageUp:   promptui.Key{Code: promptui.KeyBackward, Display: promptui.KeyBackwardDisplay},
		PageDown: promptui.Key{Code: promptui.KeyForward, Display: promptui.KeyForwardDisplay},
	}
	c := &readline.Config{}
	err = c.Init()
	if err != nil {
		return "", err
	}
	if runtime.GOOS != "windows" {
		c.Stdout = &stdout{}
	}

	c.Stdin = readline.NewCancelableStdin(c.Stdin)

	c.HistoryLimit = -1
	c.UniqueEditLine = true

	rl, err := readline.NewEx(c)
	if err != nil {
		return "", err
	}

	sb := screenbuf.New(rl)

	c.SetListener(func(line []rune, pos int, key rune) ([]rune, int, bool) {
		switch {
		case key == promptui.KeyEnter:
			return nil, 0, true
		case key == s.keys.Next.Code:
			nextItemIndex := l.Index() + 1
			if s.size > nextItemIndex && !s.items[nextItemIndex].Enable {
				l.Next()
			}
			l.Next()
		case key == s.keys.Prev.Code:
			currentIdx := l.Index()
			prevItemIndex := currentIdx - 1
			foundNewActive := false
			for prevItemIndex > -1 {
				l.Prev()
				if !s.items[prevItemIndex].Enable {
					prevItemIndex--
					continue
				}
				foundNewActive = true
				break
			}
			if !foundNewActive {
				l.SetCursor(currentIdx)
			}

		case key == s.keys.PageUp.Code:
			l.PageUp()
		case key == s.keys.PageDown.Code:
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
				output = append(output, render(s.oktetoTemplates.active, item)...)
			} else {
				output = append(output, render(s.oktetoTemplates.inactive, item)...)
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
		return "", err
	}

	items, idx := l.Items()
	item := items[idx]

	sb.Reset()
	sb.Write(render(s.oktetoTemplates.selected, item))
	sb.Flush()

	rl.Write([]byte(showCursor))
	rl.Close()

	return s.items[l.Index()].Name, err
}

func (s *OktetoSelector) prepareTemplates() error {
	tpls := s.oktetoTemplates
	if tpls == nil {
		tpls = &oktetoTemplates{}
	}

	tpls.FuncMap = s.templates.FuncMap

	if s.templates.Label == "" {
		s.templates.Label = fmt.Sprintf("%s {{.}}: ", promptui.IconInitial)
	}

	tpl, err := template.New("").Funcs(tpls.FuncMap).Parse(s.templates.Label)
	if err != nil {
		return err
	}

	tpls.label = tpl

	if s.templates.Active == "" {
		s.templates.Active = fmt.Sprintf("%s {{ . | underline }}", promptui.IconSelect)
	}

	tpl, err = template.New("").Funcs(tpls.FuncMap).Parse(s.templates.Active)
	if err != nil {
		return err
	}

	tpls.active = tpl

	if s.templates.Inactive == "" {
		s.templates.Inactive = "  {{.}}"
	}

	tpl, err = template.New("").Funcs(tpls.FuncMap).Parse(s.templates.Inactive)
	if err != nil {
		return err
	}

	tpls.inactive = tpl

	if s.templates.Selected == "" {
		s.templates.Selected = fmt.Sprintf(`{{ %q | green }} {{ . | faint }}`, promptui.IconGood)
	}

	tpl, err = template.New("").Funcs(tpls.FuncMap).Parse(s.templates.Selected)
	if err != nil {
		return err
	}
	tpls.selected = tpl

	if s.templates.Details != "" {
		tpl, err = template.New("").Funcs(tpls.FuncMap).Parse(s.templates.Details)
		if err != nil {
			return err
		}

		tpls.details = tpl
	}

	if s.templates.Help == "" {
		s.templates.Help = fmt.Sprintf(`{{ "Use the arrow keys to navigate:" | faint }} {{ .NextKey | faint }} ` +
			`{{ .PrevKey | faint }} {{ .PageDownKey | faint }} {{ .PageUpKey | faint }} ` +
			`{{ if .Search }} {{ "and" | faint }} {{ .SearchKey | faint }} {{ "toggles search" | faint }}{{ end }}`)
	}

	tpl, err = template.New("").Funcs(tpls.FuncMap).Parse(s.templates.Help)
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

	s.oktetoTemplates = tpls

	return nil
}

func (s *OktetoSelector) renderDetails(item interface{}) [][]byte {
	if s.oktetoTemplates.details == nil {
		return nil
	}

	var buf bytes.Buffer
	w := ansiterm.NewTabWriter(&buf, 0, 0, 8, ' ', 0)

	err := s.oktetoTemplates.details.Execute(w, item)
	if err != nil {
		fmt.Fprintf(w, "%v", item)
	}

	w.Flush()

	output := buf.Bytes()

	return bytes.Split(output, []byte("\n"))
}

func (s *OktetoSelector) renderLabel(sb *screenbuf.ScreenBuf) {
	width, _, _ := term.GetSize(int(os.Stdout.Fd()))
	for _, labelLine := range strings.Split(s.Label, "\n") {
		if width == 0 {
			labelLineBytes := render(s.oktetoTemplates.label, labelLine)
			sb.Write(labelLineBytes)
		} else {
			lines := len(labelLine) / width
			lastChar := 0
			for i := 0; i < lines+1; i++ {
				if lines == i {
					midLine := labelLine[lastChar:]
					labelLineBytes := render(s.oktetoTemplates.label, midLine)
					sb.Write(labelLineBytes)
					break
				}
				midLine := labelLine[lastChar : lastChar+width]
				labelLineBytes := render(s.oktetoTemplates.label, midLine)
				sb.Write(labelLineBytes)
				lastChar += width
			}
		}
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
		NextKey:     s.keys.Next.Display,
		PrevKey:     s.keys.Prev.Display,
		PageDownKey: s.keys.PageDown.Display,
		PageUpKey:   s.keys.PageUp.Display,
		SearchKey:   s.keys.Search.Display,
	}

	return render(s.oktetoTemplates.help, keys)
}

func optionsToSelectorItem(options []string) []selectorItem {
	items := make([]selectorItem, 0)
	for _, option := range options {
		enable := true
		if option == "" {
			enable = false
		}
		items = append(items, selectorItem{
			Name:   option,
			Label:  option,
			Enable: enable,
		})
	}
	return items
}
