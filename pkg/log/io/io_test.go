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
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIOControllerInitialisation(t *testing.T) {
	l := NewIOController()
	require.NotNil(t, l)
	require.Equal(t, os.Stdout, l.out.out)
	require.Equal(t, os.Stdin, l.in.in)

}

func TestGetters(t *testing.T) {
	l := NewIOController()
	require.NotNil(t, l.In())
	require.IsType(t, &InputController{}, l.In())

	require.NotNil(t, l.Out())
	require.IsType(t, &OutputController{}, l.Out())

	require.NotNil(t, l.Logger())
	require.IsType(t, &slog.Logger{}, l.Logger())
}

func TestSetLevel(t *testing.T) {
	l := NewIOController()
	defaultLevel := l.logger.slogLeveler.Level()
	assert.Equal(t, defaultLevel, slog.LevelInfo)

	l.SetLevel("debug")
	require.Equal(t, slog.LevelDebug, l.logger.slogLeveler.Level())
}

func TestSetOutputFormat(t *testing.T) {
	l := NewIOController()
	assert.IsType(t, &logrus.TextFormatter{}, l.logger.logrusFormatter)
	assert.IsType(t, &ttyFormatter{}, l.out.formatter)

	l.SetOutputFormat("plain")
	assert.IsType(t, &logrus.TextFormatter{}, l.logger.logrusFormatter)
	assert.IsType(t, &plainFormatter{}, l.out.formatter)

	l.SetOutputFormat("json")
	assert.IsType(t, &logrusJSONFormatter{}, l.logger.logrusFormatter)
	assert.IsType(t, &jsonFormatter{}, l.out.formatter)
}

func TestStage(t *testing.T) {
	l := NewIOController()

	l.SetStage("test")
	l.SetOutputFormat("json")
	l.SetStage("test")
	assert.Equal(t, "test", l.logger.logrusFormatter.(*logrusJSONFormatter).stage)
}
