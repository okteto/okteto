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
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	// errEmptyStage is returned when the stage is empty
	errEmptyStage = errors.New("empty stage")

	// errEmptyMsg is returned when the message is empty
	errEmptyMsg = errors.New("empty message")
)

// logrusJSONFormatter is a logrus formatter that adds a stage field
type logrusJSONFormatter struct {
	stage string
}

// newLogrusJSONFormatter creates a new logrusJSONFormatter
func newLogrusJSONFormatter() *logrusJSONFormatter {
	return &logrusJSONFormatter{}
}

// SetStage sets the stage
func (f *logrusJSONFormatter) SetStage(stage string) {
	f.stage = stage
}

// Format formats the message
func (f *logrusJSONFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	level := strings.ToLower(entry.Level.String())

	if f.stage == "" {
		return nil, errEmptyStage
	}
	if entry.Message == "" {
		return nil, errEmptyMsg
	}
	outputJSON := &jsonMessage{
		Level:     level,
		Timestamp: time.Now().Unix(),
		Stage:     f.stage,
		Message:   entry.Message,
	}
	messageJSON, err := json.Marshal(outputJSON)
	if err != nil {
		return nil, err
	}
	messageJSON = []byte(string(messageJSON) + "\n")
	return messageJSON, nil
}
