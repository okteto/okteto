// Copyright 2023 The Okteto Authors
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

package io

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/sirupsen/logrus"
)

// formatter is the interface for the formatters
type formatter interface {
	format(msg string) ([]byte, error)
}

// ttyFormatter is the formatter for the tty logs
type ttyFormatter struct{}

// newTTYFormatter creates a new TTYFormatter
func newTTYFormatter() *ttyFormatter {
	return &ttyFormatter{}
}

// Format formats the message for the tty
func (f *ttyFormatter) format(msg string) ([]byte, error) {
	return []byte(msg), nil
}

// ansiPattern is the regex for removing the ansi characters
const ansiPattern = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"

// ansiRegex is the regex for removing the ansi characters
var ansiRegex = regexp.MustCompile(ansiPattern)

// plainFormatter is the formatter for the plain logs
type plainFormatter struct{}

// newPlainFormatter creates a new PlainFormatter
func newPlainFormatter() *plainFormatter {
	return &plainFormatter{}
}

// Format formats the message for the plain
func (f *plainFormatter) format(msg string) ([]byte, error) {
	return ansiRegex.ReplaceAll([]byte(msg), []byte("")), nil
}

// jsonFormatter is the formatter for the json logs
type jsonFormatter struct {
	logrusFormatter *logrusJSONFormatter
	stage           string
}

// jsonMessage represents the json message
type jsonMessage struct {
	Level     string `json:"level"`
	Stage     string `json:"stage"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

// newJSONFormatter creates a new JSONFormatter
func newJSONFormatter() *jsonFormatter {
	return &jsonFormatter{
		logrusFormatter: newLogrusJSONFormatter(),
	}
}

// SetStage sets the stage of the logger
func (f *jsonFormatter) SetStage(stage string) {
	f.logrusFormatter.SetStage(stage)
	f.stage = stage
}

// Format formats the message for the json
func (f *jsonFormatter) format(msg string) ([]byte, error) {
	msg = strings.TrimRightFunc(msg, unicode.IsSpace)
	entry := &logrus.Entry{
		Message: msg,
		Level:   logrus.InfoLevel,
	}
	messageJSON, err := f.logrusFormatter.Format(entry)
	if err != nil {
		return nil, err
	}
	return messageJSON, nil
}
