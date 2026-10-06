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
	"github.com/stretchr/testify/require"
)

type safeLineRecorder struct {
	lines []string
	mu    sync.Mutex
}

func (r *safeLineRecorder) record(level string) func(line string) {
	return func(line string) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.lines = append(r.lines, level+line)
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
