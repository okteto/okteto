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
	"bytes"
	"log/slog"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestParseLevel(t *testing.T) {
	tt := []struct {
		expectedErr error
		name        string
		level       string
		expectedLvl slog.Level
	}{
		{
			name:        "info",
			level:       "info",
			expectedLvl: slog.LevelInfo,
			expectedErr: nil,
		},
		{
			name:        "debug",
			level:       "debug",
			expectedLvl: slog.LevelDebug,
			expectedErr: nil,
		},
		{
			name:        "warn",
			level:       "warn",
			expectedLvl: slog.LevelWarn,
			expectedErr: nil,
		},
		{
			name:        "error",
			level:       "error",
			expectedLvl: slog.LevelError,
			expectedErr: nil,
		},
		{
			name:        "invalid",
			level:       "invalid",
			expectedLvl: slog.LevelInfo,
			expectedErr: &InvalidLogLevelError{level: "invalid"},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			lvl, err := parseLevel(tc.level)
			require.Equal(t, tc.expectedLvl, lvl)
			if tc.expectedErr == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.expectedErr.Error())
			}
		})
	}
}

type fakeFormatter struct {
}

func (fakeFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	return []byte(entry.Message), nil
}

func TestLogger(t *testing.T) {
	ol := newOktetoLogger()
	var buf bytes.Buffer
	ol.logrusLogger.Out = &buf

	ol.logrusLogger.Formatter = fakeFormatter{}

	ol.SetLevel(DebugLevel)

	tc := []struct {
		name     string
		msg      string
		expected string
		args     []any
	}{
		{
			name:     "info",
			msg:      "info",
			expected: "info",
		},
		{
			name:     "infof",
			msg:      "infof: %s",
			args:     []any{"foo"},
			expected: "infof: foo",
		},
		{
			name:     "debug",
			msg:      "debug",
			expected: "debug",
		},
		{
			name:     "debugf",
			msg:      "debugf: %s",
			args:     []any{"foo"},
			expected: "debugf: foo",
		},
	}
	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			switch tt.name {
			case "info":
				ol.Info(tt.msg)
			case "infof":
				ol.Infof(tt.msg, tt.args...)
			case "debug":
				ol.Debug(tt.msg)
			case "debugf":
				ol.Debugf(tt.msg, tt.args...)
			}
			require.Equal(t, tt.expected, buf.String())
			buf.Reset()
		})
	}
}
