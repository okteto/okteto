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

package displayer

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
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

var (
	spinnerChars = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	cursorUp     = "\x1b[1A"
	resetLine    = "\x1b[0G"
)

// TTYDisplayer displays with a screenbuff
type TTYDisplayer struct {
	stdoutScanner *bufio.Scanner
	stderrScanner *bufio.Scanner
	screenbuf     *screenbuf.ScreenBuf

	command        string
	err            error
	linesToDisplay []string
	numberOfLines  int

	commandContext context.Context
	cancel         context.CancelFunc

	isBuilding            bool
	buildingpreviousLines int
}

func newTTYDisplayer(stdout, stderr io.Reader) *TTYDisplayer {
	var (
		stdoutScanner *bufio.Scanner
		stderrScanner *bufio.Scanner
	)
	if stdout != nil {
		stdoutScanner = bufio.NewScanner(stdout)
	}
	if stderr != nil {
		stderrScanner = bufio.NewScanner(stderr)
	}

	return &TTYDisplayer{
		numberOfLines:  25,
		linesToDisplay: []string{},

		stdoutScanner: stdoutScanner,
		stderrScanner: stderrScanner,
		screenbuf:     screenbuf.New(os.Stdout),
	}
}

// Display displays a
func (d *TTYDisplayer) Display(commandName string) {
	d.command = commandName

	d.hideCursor()
	d.commandContext, d.cancel = context.WithCancel(context.Background())
	go d.displayCommand()
	if d.stdoutScanner != nil {
		go d.displayStdout()
	}
	if d.stderrScanner != nil {
		go d.displayStderr()
	}
}

func (d *TTYDisplayer) displayCommand() {
	t := time.NewTicker(50 * time.Millisecond)
	for {
		for i := 0; i < len(spinnerChars); i++ {
			select {
			case <-t.C:
				commandLine := renderCommand(spinnerChars[i], d.command)
				d.screenbuf.Write(commandLine)
				lines := renderLines(d.linesToDisplay)
				for _, line := range lines {
					d.screenbuf.Write([]byte(line))
				}
				d.screenbuf.Flush()
			case <-d.commandContext.Done():
				return
			}

		}
	}
}

func (d *TTYDisplayer) displayStdout() {
	for d.stdoutScanner.Scan() {
		select {
		case <-d.commandContext.Done():
		default:
			line := strings.TrimSpace(d.stdoutScanner.Text())
			if isTopDisplay(line) {
				prevState := d.isBuilding
				d.isBuilding = checkIfIsBuildingLine(line)
				if d.isBuilding && d.isBuilding != prevState {
					d.buildingpreviousLines = len(d.linesToDisplay)
				}
				sanitizedLine := strings.ReplaceAll(line, cursorUp, "")
				sanitizedLine = strings.ReplaceAll(sanitizedLine, resetLine, "")
				d.linesToDisplay = append(d.linesToDisplay[:d.buildingpreviousLines-1], sanitizedLine)
			} else {
				if len(d.linesToDisplay) == d.numberOfLines {
					d.linesToDisplay = d.linesToDisplay[1:]
				}
				d.linesToDisplay = append(d.linesToDisplay, line)
			}
			if os.Stdout == oktetoLog.GetOutput() {
				oktetoLog.AddToBuffer(oktetoLog.InfoLevel, line)
			}
			continue
		}
		break
	}
	if d.stdoutScanner.Err() != nil {
		oktetoLog.Infof("Error reading command output: %s", d.stdoutScanner.Err().Error())
	}
}

func checkIfIsBuildingLine(line string) bool {
	if strings.Contains(line, "Building") {
		return !strings.Contains(line, "FINISHED")
	}
	return false
}

func (d *TTYDisplayer) displayStderr() {
	for d.stderrScanner.Scan() {
		select {
		case <-d.commandContext.Done():
		default:
			line := strings.TrimSpace(d.stderrScanner.Text())
			d.err = errors.New(line)
			if len(d.linesToDisplay) == d.numberOfLines {
				d.linesToDisplay = d.linesToDisplay[1:]
			}
			d.linesToDisplay = append(d.linesToDisplay, line)
			if os.Stdout == oktetoLog.GetOutput() {
				oktetoLog.AddToBuffer(oktetoLog.WarningLevel, line)
			}
			continue
		}
		break
	}
	if d.stderrScanner.Err() != nil {
		oktetoLog.Infof("Error reading command output: %s", d.stderrScanner.Err().Error())
	}
}

func isTopDisplay(line string) bool {
	return strings.Contains(line, fmt.Sprintf("%s%s", cursorUp, resetLine))
}

// CleanUp collapses and stop displaying
func (d *TTYDisplayer) CleanUp(err error) {
	if d.screenbuf == nil {
		return
	}
	if d.command == "" {
		return
	}

	var message []byte
	if err == nil {
		message = renderSuccessCommand(d.command)
		d.screenbuf.Reset()
		d.screenbuf.Clear()
		d.screenbuf.Flush()
		d.screenbuf.Write(message)
		d.screenbuf.Flush()
	} else {
		message = renderFailCommand(d.command, err)
		d.screenbuf.Reset()
		d.screenbuf.Clear()
		d.screenbuf.Flush()
		d.screenbuf.Write(message)
		lines := renderLines(d.linesToDisplay)
		for _, line := range lines {
			d.screenbuf.Write([]byte(line))
		}
		d.screenbuf.Flush()
	}
	d.cancel()
	<-d.commandContext.Done()
	d.reset()
	d.showCursor()

}

func renderCommand(spinnerChar, command string) []byte {
	commandTemplate := fmt.Sprintf(` %s {{ . | oktetoblue }}: `, spinnerChar)
	funcMap := promptui.FuncMap
	funcMap["oktetoblue"] = oktetoLog.BlueString
	tpl, err := template.New("").Funcs(funcMap).Parse(commandTemplate)
	if err != nil {
		return []byte{}
	}
	command = fmt.Sprintf(" Running '%s'", command)
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

func (*TTYDisplayer) hideCursor() {
	if runtime.GOOS != "windows" {
		fmt.Print("\033[?25l")
	}
}

func (*TTYDisplayer) showCursor() {
	if runtime.GOOS != "windows" {
		fmt.Print("\033[?25h")
	}
}

func (d *TTYDisplayer) reset() {

	d.command = ""
	d.err = nil

	d.linesToDisplay = []string{}
	d.screenbuf.Reset()
}
