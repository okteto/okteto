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

package build

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/util/progress/progressui"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	digest "github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// newTestMultiplexer returns a multiplexer writing to a temporary file. The shared
// display uses plain mode so its output can be asserted.
func newTestMultiplexer(t *testing.T, progress string, services []string) (*Multiplexer, *os.File) {
	t.Helper()
	out, err := os.Create(filepath.Join(t.TempDir(), "out"))
	require.NoError(t, err)
	t.Cleanup(func() { out.Close() })
	m, err := newMultiplexer(io.NewIOController(), progress, services, out, progressui.PlainMode)
	require.NoError(t, err)
	return m, out
}

// completedStatus is a status with one finished step named name
func completedStatus(id, name string) *client.SolveStatus {
	now := time.Now()
	return &client.SolveStatus{
		Vertexes: []*client.Vertex{{Digest: digest.FromString(id), Name: name, Started: &now, Completed: &now}},
	}
}

// display sends statuses to the display of svc and waits for it to finish
func display(t *testing.T, m *Multiplexer, svc string, statuses ...*client.SolveStatus) {
	t.Helper()
	ch := make(chan *client.SolveStatus)
	done := make(chan error, 1)
	go func() { done <- m.services[svc].display(ch, make(chan error, 1)) }()
	for _, ss := range statuses {
		ch <- ss
	}
	close(ch)
	require.NoError(t, <-done)
}

// closeWithin closes m and fails the test if it doesn't return in time
func closeWithin(t *testing.T, m *Multiplexer) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- m.Close() }()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("multiplexer Close did not return")
	}
}

func readOutput(t *testing.T, out *os.File) string {
	t.Helper()
	content, err := os.ReadFile(out.Name())
	require.NoError(t, err)
	return string(content)
}

func TestMultiplexerTTYPrefixesSteps(t *testing.T) {
	m, out := newTestMultiplexer(t, "tty", []string{"api", "web"})

	display(t, m, "api", completedStatus("api-1", "RUN go build"))
	display(t, m, "web", completedStatus("web-1", "RUN npm ci"))
	closeWithin(t, m)

	output := readOutput(t, out)
	require.Contains(t, output, "[api] RUN go build")
	require.Contains(t, output, "[web] RUN npm ci")
}

func TestMultiplexerTTYDoesNotMutateStatus(t *testing.T) {
	m, _ := newTestMultiplexer(t, "tty", []string{"api"})
	ss := completedStatus("api-1", "RUN go build")

	display(t, m, "api", ss)
	closeWithin(t, m)

	require.Equal(t, "RUN go build", ss.Vertexes[0].Name)
}

func TestMultiplexerTTYServiceCanSolveTwice(t *testing.T) {
	m, out := newTestMultiplexer(t, "tty", []string{"api"})

	display(t, m, "api", completedStatus("api-1", "RUN first attempt"))
	display(t, m, "api", completedStatus("api-2", "RUN second attempt"))
	m.Finish("api")
	closeWithin(t, m)

	require.Contains(t, readOutput(t, out), "[api] RUN second attempt")
}

func TestMultiplexerCloseEndsDisplayWhenServicesNeverBuilt(t *testing.T) {
	m, _ := newTestMultiplexer(t, "tty", []string{"api", "web"})

	display(t, m, "api", completedStatus("api-1", "RUN go build"))

	closeWithin(t, m)
}

func TestMultiplexerTTYShowsMessagesAfterDisplay(t *testing.T) {
	m, out := newTestMultiplexer(t, "tty", []string{"api"})

	m.IO("api").Out().Println("Image 'api' successfully pushed")
	display(t, m, "api", completedStatus("api-1", "RUN go build"))
	closeWithin(t, m)

	output := readOutput(t, out)
	require.Greater(t, strings.Index(output, "Image 'api' successfully pushed"), strings.Index(output, "[api] RUN go build"))
}

func TestMultiplexerPlainPrefixesEveryLine(t *testing.T) {
	m, out := newTestMultiplexer(t, "plain", []string{"api"})

	display(t, m, "api", completedStatus("api-1", "RUN go build"))
	closeWithin(t, m)

	for _, line := range strings.Split(strings.TrimSpace(readOutput(t, out)), "\n") {
		require.True(t, strings.HasPrefix(line, "[api] "), line)
	}
}

func TestMultiplexerPlainHoldsMessagesUntilClose(t *testing.T) {
	m, out := newTestMultiplexer(t, "plain", []string{"api"})

	m.IO("api").Out().Println("Image 'api' successfully pushed")

	require.NotContains(t, readOutput(t, out), "successfully pushed")
	closeWithin(t, m)
}

func TestMultiplexerPlainShowsMessagesAfterProgress(t *testing.T) {
	m, out := newTestMultiplexer(t, "plain", []string{"api", "web"})

	m.IO("api").Out().Println("Image 'api' successfully pushed")
	display(t, m, "api", completedStatus("api-1", "RUN go build"))
	display(t, m, "web", completedStatus("web-1", "RUN npm ci"))
	closeWithin(t, m)

	output := readOutput(t, out)
	require.Greater(t, strings.Index(output, "Image 'api' successfully pushed"), strings.Index(output, "[web] #1 RUN npm ci"))
}

func TestMultiplexerFinishIsIdempotent(t *testing.T) {
	m, _ := newTestMultiplexer(t, "tty", []string{"api"})

	m.Finish("api")
	m.Finish("api")

	closeWithin(t, m)
}

func TestMultiplexerTTYPrefixesLogBuffer(t *testing.T) {
	oktetoLog.Init(logrus.WarnLevel)
	// Lines are only buffered while a stage is set
	oktetoLog.SetStage("Building services")
	t.Cleanup(func() { oktetoLog.SetStage("") })
	m, _ := newTestMultiplexer(t, "tty", []string{"api"})

	display(t, m, "api", completedStatus("api-1", "RUN go build"))
	closeWithin(t, m)

	require.Contains(t, oktetoLog.GetOutputBuffer().String(), "[api] #1 RUN go build")
}

func TestPrefixWriterJoinsPartialWrites(t *testing.T) {
	var out bytes.Buffer
	w := &prefixWriter{prefix: "[api] ", out: &out}

	_, err := w.Write([]byte("#3 text"))
	require.NoError(t, err)
	_, err = w.Write([]byte("\n"))
	require.NoError(t, err)

	require.Equal(t, "[api] #3 text\n", out.String())
}

func TestPrefixWriterSkipsEmptyLines(t *testing.T) {
	var out bytes.Buffer
	w := &prefixWriter{prefix: "[api] ", out: &out}

	_, err := w.Write([]byte("#1 DONE 0.0s\n\n"))

	require.NoError(t, err)
	require.Equal(t, "[api] #1 DONE 0.0s\n", out.String())
}

func TestPrefixWriterFlushWritesIncompleteLine(t *testing.T) {
	var out bytes.Buffer
	w := &prefixWriter{prefix: "[api] ", out: &out}

	_, err := w.Write([]byte("#3 partial"))
	require.NoError(t, err)
	require.NoError(t, w.flush())

	require.Equal(t, "[api] #3 partial\n", out.String())
}

func TestPrefixWriterReportsBytesWritten(t *testing.T) {
	w := &prefixWriter{prefix: "[api] ", out: &bytes.Buffer{}}

	n, err := w.Write([]byte("#3 text"))

	require.NoError(t, err)
	require.Equal(t, len("#3 text"), n)
}

func useLogFormat(t *testing.T, format string) {
	t.Helper()
	previous := oktetoLog.GetOutputFormat()
	oktetoLog.SetOutputFormat(format)
	t.Cleanup(func() { oktetoLog.SetOutputFormat(previous) })
}

func TestBuildOutputModeUsesLogFormatWhenNoneIsRequested(t *testing.T) {
	useLogFormat(t, oktetoLog.JSONFormat)

	require.Equal(t, oktetoLog.JSONFormat, buildOutputMode(""))
}

func TestBuildOutputModeKeepsTheRequestedMode(t *testing.T) {
	useLogFormat(t, oktetoLog.JSONFormat)

	require.Equal(t, oktetoLog.PlainFormat, buildOutputMode(oktetoLog.PlainFormat))
}

func TestNewMultiplexerFollowsTheLogFormat(t *testing.T) {
	useLogFormat(t, oktetoLog.JSONFormat)

	m, err := NewMultiplexer(io.NewIOController(), "", []string{"api"})

	require.NoError(t, err)
	require.Nil(t, m.printer)
	closeWithin(t, m)
}
