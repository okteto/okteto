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
