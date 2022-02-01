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
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
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

	commandContext context.Context

	isBuilding            bool
	buildingpreviousLines int
}

var (
	spinnerChars = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	cursorUp     = "\x1b[1A"
	resetLine    = "\x1b[0G"
)

func newTTYExecutor() *ttyExecutor {
	return &ttyExecutor{
		numberOfLines:  25,
		linesToDisplay: []string{},
	}
}

func (e *ttyExecutor) display(command string) {
	e.command = command

	e.hideCursor()
	e.commandContext = context.Background()
	go e.displayCommand()
	go e.displayStdout()
	go e.displayStderr()

}

func (e *ttyExecutor) displayCommand() {
	t := time.NewTicker(50 * time.Millisecond)
	for {
		for i := 0; i < len(spinnerChars); i++ {
			select {
			case <-t.C:
				commandLine := renderCommand(spinnerChars[i], e.command)
				e.screenbuf.Write(commandLine)
				lines := renderLines(e.linesToDisplay)
				for _, line := range lines {
					e.screenbuf.Write([]byte(line))
				}
				e.screenbuf.Flush()
			case <-e.commandContext.Done():
				return
			}

		}
	}
}

func (e *ttyExecutor) displayStdout() {
	for e.stdoutScanner.Scan() {
		select {
		case <-e.commandContext.Done():
			break
		default:
			line := strings.TrimSpace(e.stdoutScanner.Text())
			if isTopDisplay(line) {
				prevState := e.isBuilding
				e.isBuilding = checkIfIsBuildingLine(line)
				if e.isBuilding && e.isBuilding != prevState {
					e.buildingpreviousLines = len(e.linesToDisplay)
				}
				sanitizedLine := strings.ReplaceAll(line, cursorUp, "")
				sanitizedLine = strings.ReplaceAll(sanitizedLine, resetLine, "")
				e.linesToDisplay = append(e.linesToDisplay[:e.buildingpreviousLines-1], sanitizedLine)

			} else {
				if len(e.linesToDisplay) == e.numberOfLines {
					e.linesToDisplay = e.linesToDisplay[1:]
				}
				e.linesToDisplay = append(e.linesToDisplay, line)
			}
			continue
		}
		break
	}
	if e.stdoutScanner.Err() != nil {
		oktetoLog.Infof("Error reading command output: %s", e.stdoutScanner.Err().Error())
	}
}

func checkIfIsBuildingLine(line string) bool {
	if strings.Contains(line, "Building") {
		return !strings.Contains(line, "FINISHED")
	}
	return false
}

func (e *ttyExecutor) displayStderr() {
	for e.stderrScanner.Scan() {
		select {
		case <-e.commandContext.Done():
			break
		default:
			line := strings.TrimSpace(e.stderrScanner.Text())
			e.err = errors.New(line)
			if len(e.linesToDisplay) == e.numberOfLines {
				e.linesToDisplay = e.linesToDisplay[1:]
			}
			e.linesToDisplay = append(e.linesToDisplay, line)
			continue
		}
		break
	}
	if e.stderrScanner.Err() != nil {
		oktetoLog.Infof("Error reading command output: %s", e.stderrScanner.Err().Error())
	}
}

func isTopDisplay(line string) bool {
	return strings.Contains(line, fmt.Sprintf("%s%s", cursorUp, resetLine))
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
		message = renderFailCommand(e.command, err)
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
	e.commandContext.Done()
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

func renderFailCommand(command string, err error) []byte {
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
		line = fmt.Sprintf("   %s", strings.TrimSpace(line))
		if width > 4 && len(line)+2 > width {
			result = append(result, render(tpl, fmt.Sprintf("%s...", line[:width-5])))
		} else if line == "" {
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

func (*ttyExecutor) hideCursor() {
	if runtime.GOOS != "windows" {
		fmt.Print("\033[?25l")
	}
}

func (*ttyExecutor) showCursor() {
	if runtime.GOOS != "windows" {
		fmt.Print("\033[?25h")
	}
}

func (e *ttyExecutor) reset() {

	e.command = ""
	e.err = nil

	e.linesToDisplay = []string{}
	e.screenbuf = nil

}
