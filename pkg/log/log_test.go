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

package log

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func Test_GetContextResource(t *testing.T) {
	var tests = []struct {
		name     string
		message  string
		expected string
		masked   []string
	}{
		{
			name:     "no banned words",
			message:  "hello this is a test",
			masked:   []string{},
			expected: "hello this is a test",
		},
		{
			name:     "one banned words to replace",
			message:  "hello this is a test",
			masked:   []string{"this"},
			expected: "hello *** is a test",
		},
		{
			name:     "one banned words to replace in-word",
			message:  "hello this is a test",
			masked:   []string{"is"},
			expected: "hello th*** *** a test",
		},
		{
			name:     "overlapping banned words to replace",
			message:  "hello this is a test",
			masked:   []string{"this", "is"},
			expected: "hello *** *** a test",
		},
		{
			name:     "multiline to replace",
			message:  "A\nB\nC",
			masked:   []string{"A\nB\nC"},
			expected: "***",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log.maskedWords = tt.masked
			EnableMasking()
			result := redactMessage(tt.message)
			assert.Equal(t, tt.expected, result)
			DisableMasking()
			result = redactMessage(tt.message)
			assert.Equal(t, tt.message, result)
		})
	}
}

func TestSetOutputFormat(t *testing.T) {
	Init(logrus.DebugLevel)
	var tests = []struct {
		expectedWriter    interface{}
		name              string
		hasSpinnerSupport bool
	}{
		{
			name:              "tty",
			expectedWriter:    &TTYWriter{},
			hasSpinnerSupport: true,
		},
		{
			name:              "plain",
			expectedWriter:    &PlainWriter{},
			hasSpinnerSupport: false,
		},
		{
			name:              "json",
			expectedWriter:    &JSONWriter{},
			hasSpinnerSupport: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetOutputFormat(tt.name)
			assert.IsType(t, tt.expectedWriter, log.writer)
			assert.Equal(t, tt.hasSpinnerSupport, log.spinner.spinnerSupport)
		})
	}
}
