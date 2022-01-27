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

package executor

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/manifoldco/promptui"
	"github.com/manifoldco/promptui/screenbuf"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"golang.org/x/term"
)

type ttyExecutor struct {
	stdoutScanner *bufio.Scanner
	stderrScanner *bufio.Scanner
	screenbuf     *screenbuf.ScreenBuf

	command        string
	err            error
	linesToDisplay []string
	numberOfLines  int

	quit chan bool
	wg   sync.WaitGroup
}

var spinnerChars = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func newTTYExecutor() *ttyExecutor {
	return &ttyExecutor{
		numberOfLines:  25,
		linesToDisplay: []string{},
		quit:           make(chan bool),
	}
}

func (e *ttyExecutor) display(command string) {
	e.command = command

	e.hideCursor()
	e.wg = sync.WaitGroup{}
	go e.displayCommand()
	go e.displayStdout()
	go e.displayStderr()

}

func (e *ttyExecutor) displayCommand() {
	e.wg.Add(1)
	for {
		for i := 0; i < len(spinnerChars); i++ {
			select {
			case <-e.quit:
				e.wg.Done()
				return
			default:

				commandLine := renderCommand(spinnerChars[i], e.command)
				e.screenbuf.Write(commandLine)
				lines := renderLines(e.linesToDisplay)
				for _, line := range lines {
					e.screenbuf.Write([]byte(line))
				}
				e.screenbuf.Flush()
				time.Sleep(50 * time.Millisecond)
			}

		}
	}
}

func (e *ttyExecutor) displayStdout() {
	e.wg.Add(1)
	for e.stdoutScanner.Scan() {
		line := strings.TrimSpace(e.stdoutScanner.Text())
		if len(e.linesToDisplay) == e.numberOfLines {
			e.linesToDisplay = e.linesToDisplay[1:]
		}
		e.linesToDisplay = append(e.linesToDisplay, line)
	}
	if e.stdoutScanner.Err() != nil {
		oktetoLog.Infof("Error reading command output: %s", e.stdoutScanner.Err().Error())
	}
	e.wg.Done()
}

func (e *ttyExecutor) displayStderr() {
	e.wg.Add(1)
	for e.stderrScanner.Scan() {
		line := strings.TrimSpace(e.stderrScanner.Text())
		e.err = errors.New(line)
		if len(e.linesToDisplay) == e.numberOfLines {
			e.linesToDisplay = e.linesToDisplay[1:]
		}
		e.linesToDisplay = append(e.linesToDisplay, line)
	}
	if e.stderrScanner.Err() != nil {
		oktetoLog.Infof("Error reading command output: %s", e.stderrScanner.Err().Error())
	}
	e.wg.Done()
}

func (e *ttyExecutor) cleanUp(err error) {
	if e.screenbuf == nil {
		return
	}
	if e.command == "" {
		return
	}

	var message []byte
	if err == nil {
		message = renderSuccessCommand(e.command)
		e.screenbuf.Reset()
		e.screenbuf.Clear()
		e.screenbuf.Flush()
		e.screenbuf.Write(message)
		e.screenbuf.Flush()
	} else {
		message = renderFailCommand(e.command, err, e.linesToDisplay)
		e.screenbuf.Reset()
		e.screenbuf.Clear()
		e.screenbuf.Flush()
		e.screenbuf.Write(message)
		lines := renderLines(e.linesToDisplay)
		for _, line := range lines {
			e.screenbuf.Write([]byte(line))
		}
		e.screenbuf.Flush()
	}
	e.quit <- true
	e.wg.Wait()
	e.reset()
	e.showCursor()
}

func renderCommand(spinnerChar, command string) []byte {
	commandTemplate := fmt.Sprintf(` %s {{ . | oktetoblue }}: `, spinnerChar)
	funcMap := promptui.FuncMap
	funcMap["oktetoblue"] = oktetoLog.BlueString
	tpl, err := template.New("").Funcs(funcMap).Parse(commandTemplate)
	if err != nil {
		return []byte{}
	}
	command = fmt.Sprintf(" Running %s", command)
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

func renderFailCommand(command string, err error, queue []string) []byte {
	message := fmt.Sprintf("%s: %s", command, err.Error())
	commandTemplate := `{{ " x " | bgRed | black }} {{ . | oktetored }}`
	funcMap := promptui.FuncMap
	funcMap["oktetored"] = oktetoLog.RedString
	tpl, err := template.New("").Funcs(funcMap).Parse(commandTemplate)
	if err != nil {
		return []byte{}
	}

	return render(tpl, message)
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
		line = strings.TrimSpace(line)
		if width > 4 && len(line)+2 > width {
			result = append(result, render(tpl, fmt.Sprintf("%s...", line[:width-5])))
		} else if len(line) == 0 {
			result = append(result, []byte(""))
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

func (e *ttyExecutor) hideCursor() {
	if runtime.GOOS != "windows" {
		fmt.Print("\033[?25l")
	}
}

func (e *ttyExecutor) showCursor() {
	if runtime.GOOS != "windows" {
		fmt.Print("\033[?25h")
	}
}

func (e *ttyExecutor) reset() {

	e.command = ""
	e.err = nil

	e.linesToDisplay = []string{}
	e.screenbuf = nil

	e.quit = make(chan bool)
}
