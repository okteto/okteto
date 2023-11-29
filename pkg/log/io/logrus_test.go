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

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestLogrusFormatter(t *testing.T) {
	formatter := newLogrusJSONFormatter()
	tt := []struct {
		expectedErr error
		name        string
		level       string
		stage       string
		message     string
		expected    string
	}{
		{
			name:        "empty stage",
			level:       "info",
			stage:       "",
			message:     "foobar",
			expectedErr: errEmptyStage,
		},
		{
			name:        "empty message",
			level:       "info",
			stage:       "some stage",
			message:     "",
			expectedErr: errEmptyMsg,
		},
		{
			name:    "simple string",
			level:   "info",
			stage:   "some stage",
			message: "foobar",
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			formatter.SetStage(tc.stage)
			lvl, err := logrus.ParseLevel(tc.level)
			require.NoError(t, err)

			bytes, err := formatter.Format(&logrus.Entry{
				Message: tc.message,
				Level:   lvl,
			})
			require.Equal(t, tc.expectedErr, err)

			if tc.expectedErr == nil {
				var jsonMsg jsonMessage
				json.Unmarshal(bytes, &jsonMsg)
				require.Equal(t, tc.level, jsonMsg.Level)
				require.Equal(t, tc.stage, jsonMsg.Stage)
				require.Equal(t, tc.message, jsonMsg.Message)
				require.NotEmpty(t, jsonMsg.Timestamp)
			}
		})
	}
}
