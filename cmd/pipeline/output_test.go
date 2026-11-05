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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/okteto/okteto/internal/test/client"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingDeployOutput records every message and log line it receives, with its kind:
// "" for log lines, "spinner: ", "success: " and "warning: "
type recordingDeployOutput struct {
	lines []string
	mu    sync.Mutex
}

func (r *recordingDeployOutput) record(line string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lines = append(r.lines, line)
}

func (r *recordingDeployOutput) recorded() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.lines...)
}

func (r *recordingDeployOutput) Spinner(msg string)  { r.record("spinner: " + msg) }
func (*recordingDeployOutput) StartSpinner()         {}
func (*recordingDeployOutput) StopSpinner()          {}
func (r *recordingDeployOutput) Success(msg string)  { r.record("success: " + msg) }
func (r *recordingDeployOutput) Warning(msg string)  { r.record("warning: " + msg) }
func (r *recordingDeployOutput) Println(line string) { r.record(line) }

func TestNewDeployOutputDefault(t *testing.T) {
	out := newDeployOutput(&DeployOptions{Name: "database"})

	require.IsType(t, globalDeployOutput{}, out)
}

func TestStreamPipelineLogsUsesTheDeployOutput(t *testing.T) {
	out := &recordingDeployOutput{}
	pc := &Command{
		okClient: &client.FakeOktetoClient{
			PipelineClient: client.NewFakePipelineClient(&client.FakePipelineResponses{}),
			StreamClient:   client.NewFakeStreamClient(&client.FakeStreamResponse{Lines: []string{"creating tables", "done"}}),
		},
	}

	err := pc.streamPipelineLogs(context.Background(), "database", "test", "action", time.Second, out)

	require.NoError(t, err)
	require.Equal(t, []string{"creating tables", "done"}, out.recorded())
}

func TestStreamPipelineLogsWarnsThroughTheDeployOutput(t *testing.T) {
	out := &recordingDeployOutput{}
	pc := &Command{
		okClient: &client.FakeOktetoClient{
			PipelineClient: client.NewFakePipelineClient(&client.FakePipelineResponses{}),
			StreamClient:   client.NewFakeStreamClient(&client.FakeStreamResponse{Warnings: []string{"reconnecting"}}),
		},
	}

	err := pc.streamPipelineLogs(context.Background(), "database", "test", "action", time.Second, out)

	require.NoError(t, err)
	require.Equal(t, []string{"warning: reconnecting"}, out.recorded())
}

// slowStreamClient prints its lines after a delay, unless the stream is cancelled before
type slowStreamClient struct {
	*client.FakeStreamClient
	lines []string
	delay time.Duration
}

func (c *slowStreamClient) PipelineLogsTo(ctx context.Context, _, _, _ string, _ time.Duration, printLine, _ func(msg string)) error {
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
	out := &recordingDeployOutput{}
	pc := newSlowStreamCommand(&slowStreamClient{lines: []string{"last line"}, delay: 100 * time.Millisecond})

	// the timeout is above the poll interval of the health check
	err := pc.waitUntilRunning(context.Background(), "db", "test", &types.Action{ID: "a", Name: "a"}, 5*time.Second, out)

	require.NoError(t, err)
	require.Contains(t, out.recorded(), "last line")
}

func TestWaitUntilRunningDoesNotWarnWhenTheStreamIsCancelled(t *testing.T) {
	previous := logsDrainTimeout
	logsDrainTimeout = 50 * time.Millisecond
	t.Cleanup(func() { logsDrainTimeout = previous })
	out := &recordingDeployOutput{}
	// the stream never reaches the end: it is cancelled after the drain timeout
	pc := newSlowStreamCommand(&slowStreamClient{delay: time.Hour})

	// the timeout is above the poll interval of the health check
	err := pc.waitUntilRunning(context.Background(), "db", "test", &types.Action{ID: "a", Name: "a"}, 5*time.Second, out)

	require.NoError(t, err)
	require.Equal(t, []string{"spinner: Waiting for containers to be healthy..."}, out.recorded())
}

// failingFinishPipelineClient fails while waiting for the action to finish
type failingFinishPipelineClient struct {
	*client.FakePipelineClient
}

func (failingFinishPipelineClient) WaitForActionToFinish(context.Context, string, string, string, time.Duration) error {
	return assert.AnError
}

// lingeringStreamClient streams until it is cancelled, and records when it has returned
type lingeringStreamClient struct {
	*client.FakeStreamClient
	returned atomic.Bool
}

func (c *lingeringStreamClient) PipelineLogsTo(ctx context.Context, _, _, _ string, _ time.Duration, _, _ func(msg string)) error {
	<-ctx.Done()
	// it takes a moment to stop, as a real stream that is still printing a line
	time.Sleep(10 * time.Millisecond)
	c.returned.Store(true)
	return ctx.Err()
}

func TestWaitUntilRunningStopsTheLogsStreamBeforeReturningAnError(t *testing.T) {
	previous := logsDrainTimeout
	logsDrainTimeout = 50 * time.Millisecond
	t.Cleanup(func() { logsDrainTimeout = previous })
	stream := &lingeringStreamClient{}
	pc := newWaitTestCommand(map[string]string{"svc": okteto.RunningStatus})
	fake := pc.okClient.(*client.FakeOktetoClient)
	fake.PipelineClient = failingFinishPipelineClient{fake.PipelineClient.(*client.FakePipelineClient)}
	fake.StreamClient = stream

	err := pc.waitUntilRunning(context.Background(), "db", "test", &types.Action{ID: "a", Name: "a"}, time.Second, globalDeployOutput{})

	require.ErrorIs(t, err, assert.AnError)
	require.True(t, stream.returned.Load())
}

func TestWaitUntilRunningKeepsTheLogLinesOfAFailure(t *testing.T) {
	out := &recordingDeployOutput{}
	// the pipeline fails before its last line, the one that explains why, reaches the stream
	pc := newSlowStreamCommand(&slowStreamClient{lines: []string{"the reason"}, delay: 100 * time.Millisecond})
	fake := pc.okClient.(*client.FakeOktetoClient)
	fake.PipelineClient = failingFinishPipelineClient{fake.PipelineClient.(*client.FakePipelineClient)}

	err := pc.waitUntilRunning(context.Background(), "db", "test", &types.Action{ID: "a", Name: "a"}, time.Second, out)

	require.ErrorIs(t, err, assert.AnError)
	require.Contains(t, out.recorded(), "the reason")
}

func TestWaitUntilRunningWarnsThroughTheDeployOutputWhenTheLogsCannotBeStreamed(t *testing.T) {
	out := &recordingDeployOutput{}
	pc := newWaitTestCommand(map[string]string{"svc": okteto.RunningStatus})
	pc.okClient.(*client.FakeOktetoClient).StreamClient = client.NewFakeStreamClient(&client.FakeStreamResponse{StreamErr: assert.AnError})

	err := pc.waitUntilRunning(context.Background(), "db", "test", &types.Action{ID: "a", Name: "a"}, 5*time.Second, out)

	require.NoError(t, err)
	require.Contains(t, out.recorded(), "warning: pipeline logs cannot be streamed due to connectivity issues")
}
