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
	"fmt"

	"log/slog"
)

// slogLeveler is the leveler for the logger
type slogLeveler struct {
	level slog.Level
}

// newSlogLevel returns a new leveler
func newSlogLevel(level slog.Level) *slogLeveler {
	return &slogLeveler{
		level: level,
	}
}

// Level returns the level of the logger
func (l *slogLeveler) Level() slog.Level {
	return l.level
}

// SetLevel sets the level of the logger
func (l *slogLeveler) SetLevel(lvl string) {
	slogLevel, err := parseLevel(lvl)
	if err == nil {
		l.level = slogLevel
	}
}

// InvalidLogLevelError is returned when the log level is invalid
type InvalidLogLevelError struct {
	Level string
}

// Error returns the error message
func (e *InvalidLogLevelError) Error() string {
	return fmt.Sprintf("invalid log level '%s'", e.Level)
}

// parseLevel transforms the level from a string to a slog.Level
func parseLevel(lvl string) (slog.Level, error) {
	switch lvl {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, &InvalidLogLevelError{Level: lvl}
	}
}
