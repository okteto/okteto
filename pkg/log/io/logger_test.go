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
	"log/slog"
	"testing"

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
