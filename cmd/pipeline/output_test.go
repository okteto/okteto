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

package pipeline

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/okteto/okteto/internal/test/client"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/require"
)

type safeLineRecorder struct {
	lines  []string
	stages []string
	mu     sync.Mutex
}

func (r *safeLineRecorder) record(level string) func(stage, line string) {
	return func(stage, line string) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.lines = append(r.lines, level+line)
		r.stages = append(r.stages, stage)
	}
}

// printer records every line with its level: "" for plain lines, "success: " and "warning: "
func (r *safeLineRecorder) printer() linePrinter {
	return linePrinter{
		println: r.record(""),
		success: r.record("success: "),
		warning: r.record("warning: "),
	}
}

func TestNewDeployOutputDefault(t *testing.T) {
	out := newDeployOutput(&DeployOptions{Name: "database"})

	require.IsType(t, globalDeployOutput{}, out)
}

func TestNewDeployOutputPrefixed(t *testing.T) {
	out := newDeployOutput(&DeployOptions{Name: "database", PrefixLogs: true})

	require.IsType(t, &prefixedDeployOutput{}, out)
	require.Equal(t, "database", out.(*prefixedDeployOutput).prefix)
	require.Equal(t, "Deploying dependency database", out.(*prefixedDeployOutput).stage)
}

func TestPrefixedDeployOutputRecordsItsOwnStage(t *testing.T) {
	recorder := &safeLineRecorder{}
	out := &prefixedDeployOutput{prefix: "database", stage: "Deploying dependency database", printer: recorder.printer()}

	out.Println("creating tables")
	out.Success("done")

	require.Equal(t, []string{"Deploying dependency database", "Deploying dependency database"}, recorder.stages)
}

func TestPrefixedDeployOutputPrintln(t *testing.T) {
	recorder := &safeLineRecorder{}
	out := &prefixedDeployOutput{prefix: "database", printer: recorder.printer()}

	out.Println("creating tables")

	require.Equal(t, []string{"[database] creating tables"}, recorder.lines)
}

func TestPrefixedDeployOutputPrefixesEveryLine(t *testing.T) {
	recorder := &safeLineRecorder{}
	out := &prefixedDeployOutput{prefix: "database", printer: recorder.printer()}

	out.Println("line 1\nline 2")

	require.Equal(t, []string{"[database] line 1", "[database] line 2"}, recorder.lines)
}

func TestPrefixedDeployOutputIgnoresTrailingNewline(t *testing.T) {
	recorder := &safeLineRecorder{}
	out := &prefixedDeployOutput{prefix: "database", printer: recorder.printer()}

	out.Println("line 1\r\nline 2\r\n")

	require.Equal(t, []string{"[database] line 1", "[database] line 2"}, recorder.lines)
}

func TestPrefixedDeployOutputStatusMessages(t *testing.T) {
	recorder := &safeLineRecorder{}
	out := &prefixedDeployOutput{prefix: "database", printer: recorder.printer()}

	out.Spinner("Waiting for repository 'database' to be deployed...")
	out.StartSpinner()
	out.StopSpinner()
	out.Success("Repository 'database' successfully deployed")
	out.Warning("pipeline logs cannot be streamed")

	require.Equal(t, []string{
		"[database] Waiting for repository 'database' to be deployed...",
		"success: [database] Repository 'database' successfully deployed",
		"warning: [database] pipeline logs cannot be streamed",
	}, recorder.lines)
}

// writeLinesConcurrently writes n lines from every output at the same time
func writeLinesConcurrently(outputs []*prefixedDeployOutput, n int) {
	var wg sync.WaitGroup
	for _, out := range outputs {
		wg.Add(1)
		go func(out *prefixedDeployOutput) {
			defer wg.Done()
			for i := 0; i < n; i++ {
				out.Println(fmt.Sprintf("line %d", i))
			}
		}(out)
	}
	wg.Wait()
}

func requireEveryLineHasOnePrefix(t *testing.T, lines []string, prefixes ...string) {
	t.Helper()
	for _, line := range lines {
		matches := 0
		for _, prefix := range prefixes {
			if strings.HasPrefix(line, fmt.Sprintf("[%s] line ", prefix)) {
				matches++
			}
		}
		require.Equal(t, 1, matches, "line %q", line)
	}
}

func TestPrefixedDeployOutputConcurrentWriters(t *testing.T) {
	recorder := &safeLineRecorder{}
	outputs := []*prefixedDeployOutput{
		{prefix: "database", printer: recorder.printer()},
		{prefix: "messaging", printer: recorder.printer()},
	}

	writeLinesConcurrently(outputs, 100)

	require.Len(t, recorder.lines, 200)
	requireEveryLineHasOnePrefix(t, recorder.lines, "database", "messaging")
}

func TestStreamPipelineLogsUsesTheDeployOutput(t *testing.T) {
	recorder := &safeLineRecorder{}
	out := &prefixedDeployOutput{prefix: "database", printer: recorder.printer()}
	pc := &Command{
		okClient: &client.FakeOktetoClient{
			PipelineClient: client.NewFakePipelineClient(&client.FakePipelineResponses{}),
			StreamClient:   client.NewFakeStreamClient(&client.FakeStreamResponse{Lines: []string{"creating tables", "done"}}),
		},
	}

	err := pc.streamPipelineLogs(context.Background(), "database", "test", "action", time.Second, out)

	require.NoError(t, err)
	require.Equal(t, []string{"[database] creating tables", "[database] done"}, recorder.lines)
}

// slowStreamClient prints its lines after a delay, unless the stream is cancelled before
type slowStreamClient struct {
	*client.FakeStreamClient
	lines []string
	delay time.Duration
}

func (c *slowStreamClient) PipelineLogsTo(ctx context.Context, _, _, _ string, _ time.Duration, printLine func(line string)) error {
	select {
	case <-time.After(c.delay):
		for _, line := range c.lines {
			printLine(line)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func newSlowStreamCommand(stream *slowStreamClient) *Command {
	pc := newWaitTestCommand(map[string]string{"svc": okteto.RunningStatus})
	pc.okClient.(*client.FakeOktetoClient).StreamClient = stream
	return pc
}

func TestWaitUntilRunningKeepsTheLastLogLines(t *testing.T) {
	recorder := &safeLineRecorder{}
	out := &prefixedDeployOutput{prefix: "db", printer: recorder.printer()}
	pc := newSlowStreamCommand(&slowStreamClient{lines: []string{"last line"}, delay: 100 * time.Millisecond})

	err := pc.waitUntilRunning(context.Background(), "db", "test", &types.Action{ID: "a", Name: "a"}, time.Second, true, out)

	require.NoError(t, err)
	require.Contains(t, recorder.lines, "[db] last line")
}

func TestWaitUntilRunningDoesNotWarnWhenTheStreamIsCancelled(t *testing.T) {
	previous := logsDrainTimeout
	logsDrainTimeout = 50 * time.Millisecond
	t.Cleanup(func() { logsDrainTimeout = previous })
	recorder := &safeLineRecorder{}
	out := &prefixedDeployOutput{prefix: "db", printer: recorder.printer()}
	// the stream never reaches the end: it is cancelled after the drain timeout
	pc := newSlowStreamCommand(&slowStreamClient{delay: time.Hour})

	err := pc.waitUntilRunning(context.Background(), "db", "test", &types.Action{ID: "a", Name: "a"}, time.Second, true, out)

	require.NoError(t, err)
	require.Empty(t, recorder.lines)
}
