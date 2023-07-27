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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ConvertToJson(t *testing.T) {
	defaultLevel := "info"
	defaultStage := "some stage"
	mockedTimestamp := int64(123)

	var tests = []struct {
		name     string
		stage    string
		level    string
		message  string
		err      error
		expected jsonMessage
	}{
		{
			name:    "empty stage",
			level:   defaultLevel,
			stage:   "",
			message: "foobar",
			err:     &json.SyntaxError{},
			expected: jsonMessage{
				Timestamp: mockedTimestamp,
			},
		},
		{
			name:    "empty message",
			level:   defaultLevel,
			stage:   defaultStage,
			message: "",
			err:     &json.SyntaxError{},
			expected: jsonMessage{
				Timestamp: mockedTimestamp,
			},
		},
		{
			name:    "simple string",
			level:   defaultLevel,
			stage:   defaultStage,
			message: "foobar",
			expected: jsonMessage{
				Level:     defaultLevel,
				Stage:     defaultStage,
				Message:   "foobar",
				Timestamp: int64(mockedTimestamp),
			},
		}, {
			name:    "leaving leading whitespace since it represents indentation",
			level:   defaultLevel,
			stage:   defaultStage,
			message: " \t\nsome indented line",
			expected: jsonMessage{
				Level:     defaultLevel,
				Stage:     defaultStage,
				Message:   " \t\nsome indented line",
				Timestamp: int64(mockedTimestamp),
			},
		}, {
			name:    "removes trailing whitespace since each line should represent an individual line",
			level:   defaultLevel,
			stage:   defaultStage,
			message: "  some indented line \t\n",
			expected: jsonMessage{
				Level:     defaultLevel,
				Stage:     defaultStage,
				Message:   "  some indented line",
				Timestamp: int64(mockedTimestamp),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := convertToJSON(tt.level, tt.stage, tt.message)
			var resultJSON jsonMessage
			err := json.Unmarshal([]byte(s), &resultJSON)
			if err != nil {
				assert.ErrorAs(t, err, &tt.err)
			}
			// Ignore timestamp in tests
			resultJSON.Timestamp = mockedTimestamp
			assert.Equal(t, tt.expected, resultJSON)
		})
	}
}
