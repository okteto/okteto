// Copyright 2021 The Okteto Authors
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

package executor

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"runtime"
	"text/template"

	"github.com/manifoldco/promptui"
	"github.com/manifoldco/promptui/screenbuf"
	"github.com/okteto/okteto/pkg/log"
	"golang.org/x/term"
)

type ttyExecutorDisplayer struct {
	stdoutScanner *bufio.Scanner
	stderrScanner *bufio.Scanner
	screenbuf     *screenbuf.ScreenBuf

	command string
	err     error

	numberOfLines int
}

func newTTYExecutorDisplayer() *ttyExecutorDisplayer {
	return &ttyExecutorDisplayer{
		numberOfLines: 25,
	}
}

func (e *ttyExecutorDisplayer) display(command string) {
	queue := []string{}
	e.hideCursor()
	commandLine := renderCommand(command)

	e.screenbuf.Write(commandLine)
	e.screenbuf.Flush()

	go func() {
		for e.stdoutScanner.Scan() {
			e.screenbuf.Write(commandLine)
			line := e.stdoutScanner.Text()
			if len(queue) == e.numberOfLines {
				queue = queue[1:]
			}
			queue = append(queue, line)
			lines := renderLines(queue)
			for _, line := range lines {
				e.screenbuf.Write([]byte(line))
			}
			e.screenbuf.Flush()
		}
		if e.stdoutScanner.Err() != nil {
			log.Infof("Error reading command output: %s", e.stdoutScanner.Err().Error())
		}

	}()

	go func() {
		for e.stderrScanner.Scan() {
			e.screenbuf.Write(commandLine)
			line := e.stderrScanner.Text()
			e.err = errors.New(line)
			if len(queue) == e.numberOfLines {
				queue = queue[1:]
			}
			queue = append(queue, line)
			lines := renderLines(queue)
			for _, line := range lines {
				e.screenbuf.Write([]byte(line))
			}
			e.screenbuf.Flush()
		}
		if e.stderrScanner.Err() != nil {
			log.Infof("Error reading command output: %s", e.stderrScanner.Err().Error())
		}
	}()
}

func (e *ttyExecutorDisplayer) cleanUp() {
	collapseTTY(e.command, e.err, e.screenbuf)
	e.screenbuf.Reset()
	e.screenbuf.Flush()
	e.showCursor()
}

func collapseTTY(command string, err error, sb *screenbuf.ScreenBuf) {
	if sb == nil {
		return
	}
	var message []byte
	if err == nil {
		message = renderSuccessCommand(command)
	} else {
		message = renderFailCommand(command)
	}
	sb.Reset()
	sb.Write(message)
	sb.Flush()
}

func renderCommand(command string) []byte {
	commandTemplate := "{{ . | blue }}: "
	tpl, err := template.New("").Funcs(promptui.FuncMap).Parse(commandTemplate)
	if err != nil {
		return []byte{}
	}
	command = fmt.Sprintf("Running %s", command)
	return render(tpl, command)
}

func renderSuccessCommand(command string) []byte {
	commandTemplate := `{{ " ✓ " | bgGreen | black }} {{ . | green }}`
	tpl, err := template.New("").Funcs(promptui.FuncMap).Parse(commandTemplate)
	if err != nil {
		return []byte{}
	}

	return render(tpl, command)
}

func renderFailCommand(command string) []byte {
	commandTemplate := `{{ " x " | bgRed | black }} {{ . | red }}`
	tpl, err := template.New("").Funcs(promptui.FuncMap).Parse(commandTemplate)
	if err != nil {
		return []byte{}
	}

	return render(tpl, command)
}

func renderLines(queue []string) [][]byte {
	lineTemplate := "{{ . | white }} "
	tpl, err := template.New("").Funcs(promptui.FuncMap).Parse(lineTemplate)
	if err != nil {
		return [][]byte{}
	}

	result := [][]byte{}
	for _, line := range queue {
		width, _, _ := term.GetSize(int(os.Stdout.Fd()))
		if width > 4 && len(line)+2 > width {
			result = append(result, render(tpl, fmt.Sprintf("%s...", line[:width-5])))
		} else {
			result = append(result, render(tpl, line))
		}

	}
	return result
}

func render(tpl *template.Template, data interface{}) []byte {
	var buf bytes.Buffer
	err := tpl.Execute(&buf, data)
	if err != nil {
		return []byte(fmt.Sprintf("%v", data))
	}
	return buf.Bytes()
}

func (e *ttyExecutorDisplayer) hideCursor() {
	if runtime.GOOS != "windows" {
		fmt.Print("\033[?25l")
	}
}

func (e *ttyExecutorDisplayer) showCursor() {
	if runtime.GOOS != "windows" {
		fmt.Print("\033[?25h")
	}
}
