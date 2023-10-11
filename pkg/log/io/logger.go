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

	sloglogrus "github.com/samber/slog-logrus"
	"github.com/sirupsen/logrus"
)

const (
	// JSONFormat represents a json logger
	JSONFormat string = "json"
)

// oktetoLogger is the logger for the CLI information and debug logs
type oktetoLogger struct {
	*slog.Logger
	slogLeveler *slogLeveler

	logrusLogger    *logrus.Logger
	logrusFormatter logrus.Formatter
}

// newOktetoLogger returns a new logger
func newOktetoLogger() *oktetoLogger {
	leveler := newSlogLevel(slog.LevelInfo)
	logrusLogger := logrus.New()
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

// SetLevel sets the level of the logger
func (ol *oktetoLogger) SetLevel(lvl string) {
	ol.slogLeveler.SetLevel(lvl)
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
