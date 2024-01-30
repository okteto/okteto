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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTTYFormatter(t *testing.T) {
	formatter := newTTYFormatter()
	msg := "text message"

	output, err := formatter.format(msg)
	assert.NoError(t, err)
	assert.Equal(t, []byte(msg), output)
}

func TestPlainFormatter(t *testing.T) {
	formatter := newPlainFormatter()

	tt := []struct {
		name     string
		msg      string
		expected string
	}{
		{
			name:     "without ansi",
			msg:      "text message",
			expected: "text message",
		},
		{
			name:     "with ansi",
			msg:      "\x1B[32mHello, \x1B[1;31mworld!\x1B[0m",
			expected: "Hello, world!",
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			output, err := formatter.format(tc.msg)
			assert.NoError(t, err)
			assert.Equal(t, []byte(tc.expected), output)
		})
	}
}

func TestJSONFormatter(t *testing.T) {
	formatter := newJSONFormatter()
	msg := "text message\n"
	stage := "stage"
	formatter.SetStage(stage)

	output, err := formatter.format(msg)
	assert.NoError(t, err)

	jsonMessage := &jsonMessage{}
	json.Unmarshal(output, jsonMessage)

	assert.Equal(t, "text message", jsonMessage.Message)
	assert.Equal(t, stage, jsonMessage.Stage)
	assert.Equal(t, "info", jsonMessage.Level)
	assert.NotEmpty(t, jsonMessage.Timestamp)

}
