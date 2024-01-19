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

	sloglogrus "github.com/samber/slog-logrus/v2"
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	// JSONFormat represents a json logger
	JSONFormat string = "json"

	// DebugLevel is the debug level
	DebugLevel = "debug"

	// InfoLevel is the info level
	InfoLevel = "info"

	// WarnLevel is the warn level
	WarnLevel = "warn"

	// ErrorLevel is the error level
	ErrorLevel = "error"
)

var (
	// levelMap transforms a slog.Level to a logrus.Level
	levelMap = map[slog.Level]logrus.Level{
		slog.LevelDebug: logrus.DebugLevel,
		slog.LevelInfo:  logrus.InfoLevel,
		slog.LevelWarn:  logrus.WarnLevel,
		slog.LevelError: logrus.ErrorLevel,
	}

	// DefaultLogLevel is the default log level
	DefaultLogLevel = slog.LevelWarn
)

// oktetoLogger is used for recording and categorizing log messages at different levels (e.g., info, debug, warning).
// These log messages can be filtered based on the log level set by the user.
// Messages with log levels lower than the user-defined log level will not be displayed to the user.
type oktetoLogger struct {
	*slog.Logger
	slogLeveler *slog.LevelVar

	logrusLogger    *logrus.Logger
	logrusFormatter logrus.Formatter
}

// newOktetoLogger returns an initialised oktetoLogger
func newOktetoLogger() *oktetoLogger {
	leveler := new(slog.LevelVar)
	leveler.Set(DefaultLogLevel)

	logrusLogger := logrus.New()
	logrusLogger.SetLevel(levelMap[DefaultLogLevel])
	logrusFormatter := &logrus.TextFormatter{}
	logrusLogger.SetFormatter(logrusFormatter)

	logger := slog.New(sloglogrus.Option{Level: leveler, Logger: logrusLogger}.NewLogrusHandler())
	return &oktetoLogger{
		slogLeveler: leveler,
		Logger:      logger,

		logrusLogger:    logrusLogger,
		logrusFormatter: logrusFormatter,
	}
}

// newFileLogger returns an initialised oktetoLogger that logs to file
func newFileLogger(logPath string) *oktetoLogger {
	leveler := new(slog.LevelVar)
	leveler.Set(slog.LevelDebug)

	logrusLogger := logrus.New()
	logrusLogger.SetLevel(levelMap[slog.LevelDebug])
	logrusFormatter := &logrus.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	}
	logrusLogger.SetFormatter(logrusFormatter)

	rolling := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    1, // megabytes
		MaxBackups: 3,
		MaxAge:     28, // days
		Compress:   true,
	}
	logrusLogger.SetOutput(rolling)

	logger := slog.New(sloglogrus.Option{Level: leveler, Logger: logrusLogger}.NewLogrusHandler())
	return &oktetoLogger{
		slogLeveler: leveler,
		Logger:      logger,

		logrusLogger:    logrusLogger,
		logrusFormatter: logrusFormatter,
	}
}

// SetLevel sets the level of the logger
func (ol *oktetoLogger) SetLevel(lvl string) {
	slogLevel, err := parseLevel(lvl)
	if err == nil {
		ol.slogLeveler.Set(slogLevel)
		ol.logrusLogger.SetLevel(levelMap[slogLevel])
	}
}

// SetOutputFormat sets the output format of the logger
func (ol *oktetoLogger) SetOutputFormat(output string) {
	switch output {
	case JSONFormat:
		ol.logrusFormatter = newLogrusJSONFormatter()
	default:
		ol.logrusFormatter = &logrus.TextFormatter{}
	}
	ol.logrusLogger.SetFormatter(ol.logrusFormatter)
}

func (ol *oktetoLogger) SetStage(stage string) {
	if v, ok := ol.logrusLogger.Formatter.(*logrusJSONFormatter); ok {
		v.SetStage(stage)
	}
}

// Infof logs an info message
func (ol *oktetoLogger) Infof(format string, args ...any) {
	ol.logrusLogger.Info(fmt.Sprintf(format, args...))
}

// Debugf logs a debug message
func (ol *oktetoLogger) Debugf(format string, args ...any) {
	ol.logrusLogger.Debug(fmt.Sprintf(format, args...))
}

// Info logs an info message
func (ol *oktetoLogger) Info(msg string) {
	ol.logrusLogger.Info(msg)
}

// Debug logs a debug message
func (ol *oktetoLogger) Debug(msg string) {
	ol.logrusLogger.Debug(msg)
}

// InvalidLogLevelError is returned when the log level is invalid
type InvalidLogLevelError struct {
	level string
}

// Error returns the error message
func (e *InvalidLogLevelError) Error() string {
	return fmt.Sprintf("invalid log level '%s'", e.level)
}

// parseLevel transforms the level from a string to a slog.Level
func parseLevel(lvl string) (slog.Level, error) {
	switch lvl {
	case DebugLevel:
		return slog.LevelDebug, nil
	case InfoLevel:
		return slog.LevelInfo, nil
	case WarnLevel:
		return slog.LevelWarn, nil
	case ErrorLevel:
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, &InvalidLogLevelError{level: lvl}
	}
}
