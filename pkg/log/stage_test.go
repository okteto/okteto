// Copyright 2026 The Okteto Authors
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
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// initLoggerWithFormat resets the logger with the given output format and the main deploy running a command
func initLoggerWithFormat(t *testing.T, format string) {
	t.Helper()
	Init(logrus.WarnLevel)
	SetOutputFormat(format)
	SetStage("main command")
	t.Cleanup(func() {
		SetStage("")
		Init(logrus.WarnLevel)
	})
}

// bufferedStages returns the stage of every line recorded in the output buffer, keyed by message
func bufferedStages(t *testing.T) map[string]string {
	t.Helper()
	stages := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(GetOutputBuffer().String()), "\n") {
		msg := jsonMessage{}
		require.NoError(t, json.Unmarshal([]byte(line), &msg))
		stages[strings.TrimSpace(msg.Message)] = msg.Stage
	}
	return stages
}

func TestPrintlnWithStage(t *testing.T) {
	for _, format := range []string{TTYFormat, PlainFormat, JSONFormat} {
		t.Run(format, func(t *testing.T) {
			initLoggerWithFormat(t, format)

			PrintlnWithStage("Deploying dependency db", "creating tables")

			require.Equal(t, "Deploying dependency db", bufferedStages(t)["creating tables"])
			require.Equal(t, "main command", log.getStage())
		})
	}
}

func TestSuccessWithStage(t *testing.T) {
	for _, format := range []string{TTYFormat, PlainFormat, JSONFormat} {
		t.Run(format, func(t *testing.T) {
			initLoggerWithFormat(t, format)

			SuccessWithStage("Deploying dependency db", "db deployed")

			require.Contains(t, stagesOfMessagesContaining(t, "db deployed"), "Deploying dependency db")
			require.NotContains(t, stagesOfMessagesContaining(t, "db deployed"), "main command")
		})
	}
}

func TestWarningWithStage(t *testing.T) {
	for _, format := range []string{TTYFormat, PlainFormat} {
		t.Run(format, func(t *testing.T) {
			initLoggerWithFormat(t, format)

			WarningWithStage("Deploying dependency db", "logs not available")

			require.Contains(t, stagesOfMessagesContaining(t, "logs not available"), "Deploying dependency db")
			require.NotContains(t, stagesOfMessagesContaining(t, "logs not available"), "main command")
		})
	}
}

func TestPrintlnWithStageRedactsSecrets(t *testing.T) {
	initLoggerWithFormat(t, TTYFormat)
	AddMaskedWord("super-secret")
	EnableMasking()
	t.Cleanup(DisableMasking)

	PrintlnWithStage("Deploying dependency db", "password super-secret")

	require.Equal(t, "Deploying dependency db", bufferedStages(t)["password ***"])
	require.NotContains(t, GetOutputBuffer().String(), "super-secret")
}

func TestWithStageInJSONStdout(t *testing.T) {
	initLoggerWithFormat(t, JSONFormat)
	stdout := &bytes.Buffer{}
	log.out.SetOutput(stdout)

	PrintlnWithStage("Deploying dependency db", "creating tables")
	SuccessWithStage("Deploying dependency db", "db deployed")
	WarningWithStage("Deploying dependency db", "logs not available")

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	require.Len(t, lines, 3)
	requireEveryJSONLineHasStage(t, lines, "Deploying dependency db")
}

func requireEveryJSONLineHasStage(t *testing.T, lines []string, stage string) {
	t.Helper()
	for _, line := range lines {
		msg := jsonMessage{}
		require.NoError(t, json.Unmarshal([]byte(line), &msg))
		require.Equal(t, stage, msg.Stage, line)
	}
}

// stagesOfMessagesContaining returns the stages of the buffered lines whose message contains text
func stagesOfMessagesContaining(t *testing.T, text string) []string {
	t.Helper()
	stages := []string{}
	for msg, stage := range bufferedStages(t) {
		if strings.Contains(msg, text) {
			stages = append(stages, stage)
		}
	}
	return stages
}
