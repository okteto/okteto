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

package deploy

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// captureDependencyOutput resets the logger with the given output format, under the dependencies stage, and
// returns a function that gives every line of its output buffer as "level: message"
func captureDependencyOutput(t *testing.T, format string) func() []string {
	t.Helper()
	oktetoLog.Init(logrus.WarnLevel)
	oktetoLog.SetOutputFormat(format)
	oktetoLog.SetStage(dependenciesStage)
	t.Cleanup(func() {
		oktetoLog.SetStage("")
		oktetoLog.Init(logrus.WarnLevel)
	})
	return func() []string {
		return bufferedLines(t)
	}
}

// bufferedLines returns every line of the output buffer as "level: message", checking it's under the dependencies stage
func bufferedLines(t *testing.T) []string {
	t.Helper()
	lines := []string{}
	for _, line := range strings.Split(strings.TrimSpace(oktetoLog.GetOutputBuffer().String()), "\n") {
		msg := struct {
			Level   string `json:"level"`
			Stage   string `json:"stage"`
			Message string `json:"message"`
		}{}
		require.NoError(t, json.Unmarshal([]byte(line), &msg), line)
		require.Equal(t, dependenciesStage, msg.Stage, line)
		lines = append(lines, fmt.Sprintf("%s: %s", msg.Level, strings.TrimRight(msg.Message, "\n")))
	}
	return lines
}

func TestDependencyOutputOfAnAwaitedDependency(t *testing.T) {
	printed := captureDependencyOutput(t, oktetoLog.JSONFormat)
	out := newDependencyOutput("db", true)

	out.Spinner("Waiting for repository 'db' to be deployed...")
	out.StartSpinner()
	out.Println("creating tables")
	out.StopSpinner()
	out.Success("Repository 'db' successfully deployed")
	out.finish(nil)

	require.Equal(t, []string{
		"info: [db] Waiting for repository 'db' to be deployed...",
		"info: [db] creating tables",
		fmt.Sprintf("info: [db] %s Repository 'db' successfully deployed", successSymbol()),
	}, printed())
}

func TestDependencyOutputOfAScheduledDependency(t *testing.T) {
	printed := captureDependencyOutput(t, oktetoLog.JSONFormat)
	out := newDependencyOutput("reports", false)

	out.Spinner("Deploying repository 'reports'...")
	out.StartSpinner()
	out.StopSpinner()
	out.Success("Repository 'reports' scheduled for deployment")
	out.finish(nil)

	require.Equal(t, []string{fmt.Sprintf("info: [reports] %s Repository 'reports' scheduled for deployment", successSymbol())}, printed())
}

func TestDependencyOutputPrefixesEveryLine(t *testing.T) {
	printed := captureDependencyOutput(t, oktetoLog.JSONFormat)
	out := newDependencyOutput("db", true)

	out.Println("line 1\r\nline 2\r\n")
	out.Warning("reconnecting\nto the logs")

	require.Equal(t, []string{
		"info: [db] line 1",
		"info: [db] line 2",
		"warn: [db]  !  reconnecting",
		"warn: [db]  !  to the logs",
	}, printed())
}

func TestDependencyOutputEndsAFailureWithItsError(t *testing.T) {
	printed := captureDependencyOutput(t, oktetoLog.JSONFormat)
	out := newDependencyOutput("reports", false)

	out.Spinner("Deploying repository 'reports'...")
	out.finish(errors.New("failed to deploy pipeline 'reports': forbidden"))

	require.Equal(t, []string{"error: [reports]  x  failed to deploy pipeline 'reports': forbidden"}, printed())
}

func TestDependencyOutputDoesNotShowAnInterruptionAsAnError(t *testing.T) {
	printed := captureDependencyOutput(t, oktetoLog.JSONFormat)
	out := newDependencyOutput("db", true)

	out.Println("db: working")
	out.finish(fmt.Errorf("wait for pipeline 'db' to finish failed: %w", oktetoErrors.ErrIntSig))

	require.Equal(t, []string{"info: [db] db: working"}, printed())
}

// printEveryLevel prints a message of every level from the output of an awaited dependency, which fails
func printEveryLevel() {
	out := newDependencyOutput("db", true)
	out.Println("creating tables")
	out.Success("deployed")
	out.Warning("reconnecting")
	out.finish(errors.New("exploded"))
}

func TestDependencyOutputInPlainOutput(t *testing.T) {
	printed := captureDependencyOutput(t, oktetoLog.PlainFormat)

	printEveryLevel()

	// the plain logger stores the lines it prints twice in the buffer, as it always has
	require.Subset(t, printed(), []string{
		"info: [db] creating tables",
		"info: [db] SUCCESS: deployed",
		"info: [db] WARNING: reconnecting",
		"info: [db] ERROR: exploded",
		"error: [db] exploded",
	})
}

func TestDependencyOutputInTTYOutput(t *testing.T) {
	printed := captureDependencyOutput(t, oktetoLog.TTYFormat)

	printEveryLevel()

	// colors are off in tests
	require.Equal(t, []string{
		"info: [db] creating tables",
		fmt.Sprintf("info: [db] %s deployed", successSymbol()),
		"info: [db]  !  reconnecting",
		"info: [db]  x  exploded",
		"error: [db] exploded",
	}, printed())
}

// writeMessagesConcurrently writes n messages of two lines from the output of every dependency at the same time
func writeMessagesConcurrently(names []string, n int) {
	var wg sync.WaitGroup
	for _, name := range names {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out := newDependencyOutput(name, true)
			for i := 0; i < n; i++ {
				out.Println(fmt.Sprintf("message %d\nmessage %d", i, i))
			}
		}()
	}
	wg.Wait()
}

// requireMessagesTogether checks that the two lines of every message are printed one after the other
func requireMessagesTogether(t *testing.T, lines []string) {
	t.Helper()
	require.Zero(t, len(lines)%2)
	for i := 0; i < len(lines); i += 2 {
		require.Equal(t, lines[i], lines[i+1], "lines %d and %d", i, i+1)
	}
}

func TestDependencyOutputKeepsTheLinesOfAMessageTogether(t *testing.T) {
	printed := captureDependencyOutput(t, oktetoLog.JSONFormat)

	writeMessagesConcurrently([]string{"db", "messaging"}, 200)

	lines := printed()
	require.Len(t, lines, 800)
	requireMessagesTogether(t, lines)
}
