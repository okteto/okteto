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

package utils

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"text/template"
	"time"

	"github.com/manifoldco/promptui"
	"github.com/manifoldco/promptui/screenbuf"
	"github.com/okteto/okteto/pkg/log"
	"golang.org/x/term"
)

type ManifestExecutor interface {
	Execute(command string, env []string) error
}

type Executor struct {
	outputMode string
}

type jsonMessage struct {
	Level     string `json:"level"`
	Stage     string `json:"stage"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

// NewExecutor returns a new executor
func NewExecutor(output string) *Executor {
	if output == "tty" && !IsSupportForTTY() {
		output = "plain"
	}
	return &Executor{
		outputMode: output,
	}
}

func IsSupportForTTY() bool {
	if runtime.GOOS != "windows" {
		return isSupportForTTY()
	}
	return false
}

// Execute executes the specified command adding `env` to the execution environment
func (e *Executor) Execute(command string, env []string) error {

	cmd := exec.Command("bash", "-c", command)
	cmd.Env = append(os.Environ(), env...)

	var scanner *bufio.Scanner

	if e.outputMode == "tty" {
		var (
			reader io.Reader
			err    error
		)
		if runtime.GOOS != "windows" {
			reader, err = startCommandTTY(cmd)
		}
		if err != nil {
			return err
		}
		scanner = bufio.NewScanner(reader)
	} else {
		r, err := cmd.StdoutPipe()
		if err != nil {
			return err
		}
		scanner = bufio.NewScanner(r)

		if err := cmd.Start(); err != nil {
			return err
		}
	}

	sb := screenbuf.New(os.Stdout)
	if e.outputMode == "plain" {
		go displayPlainOutput(scanner)
	} else if e.outputMode == "json" {
		go displayJSONOutput(scanner, command)
	} else {
		go displayTTYOutput(scanner, command, sb)
	}

	err := cmd.Wait()
	if e.outputMode == "tty" {
		collapseTTY(command, err, sb)
	}
	return err
}

func displayPlainOutput(scanner *bufio.Scanner) {
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Println(line)
	}
}
func displayJSONOutput(scanner *bufio.Scanner, command string) {

	for scanner.Scan() {
		line := scanner.Text()
		level := "info"
		if isErrorLine(line) {
			level = "error"
		}
		messageStruct := jsonMessage{
			Level:     level,
			Message:   line,
			Stage:     command,
			Timestamp: time.Now().Unix(),
		}
		message, _ := json.Marshal(messageStruct)
		fmt.Println(string(message))
	}
}

func displayTTYOutput(scanner *bufio.Scanner, command string, sb *screenbuf.ScreenBuf) {
	queue := []string{}
	for scanner.Scan() {
		commandLine := renderCommand(command)
		sb.Write(commandLine)
		line := scanner.Text()
		if len(queue) == 10 {
			queue = queue[1:]
		}
		queue = append(queue, line)
		lines := renderLines(queue)
		for _, line := range lines {
			sb.Write([]byte(line))
		}
		sb.Flush()
	}
	if scanner.Err() != nil {
		log.Infof("Error reading command output: %s", scanner.Err().Error())
	}
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
func isErrorLine(text string) bool {
	return strings.HasPrefix(text, " x ")
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
