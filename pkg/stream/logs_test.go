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

package stream

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// newOpenStreamServer sends the lines and keeps the connection open without sending anything else,
// like a server that has nothing more to say
func newOpenStreamServer(t *testing.T, lines ...string) *httptest.Server {
	t.Helper()
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, line := range lines {
			fmt.Fprintf(w, "data: %s\n", line)
		}
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(func() {
		close(release)
		server.Close()
	})
	return server
}

// getLogsAsync runs GetLogsFromURL and returns a channel with its result
func getLogsAsync(ctx context.Context, url string, handler handleLineFn) <-chan error {
	result := make(chan error, 1)
	go func() {
		result <- GetLogsFromURL(ctx, http.DefaultClient, url, handler, time.Minute)
	}()
	return result
}

func requireReturnsWithin(t *testing.T, result <-chan error, d time.Duration) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(d):
		require.FailNow(t, "GetLogsFromURL didn't return")
		return nil
	}
}

func TestGetLogsFromURLReturnsAtTheDoneMessage(t *testing.T) {
	server := newOpenStreamServer(t, "line", "EOF")
	lines := []string{}

	result := getLogsAsync(context.Background(), server.URL, func(line string) bool {
		lines = append(lines, line)
		return line == "EOF"
	})

	require.NoError(t, requireReturnsWithin(t, result, 5*time.Second))
	require.Equal(t, []string{"line", "EOF"}, lines)
}

func TestGetLogsFromURLReturnsWhenCancelledWhileWaitingForLines(t *testing.T) {
	server := newOpenStreamServer(t, "line")
	ctx, cancel := context.WithCancel(context.Background())
	received := make(chan struct{}, 1)

	result := getLogsAsync(ctx, server.URL, func(string) bool {
		received <- struct{}{}
		return false
	})
	<-received
	cancel()

	require.ErrorIs(t, requireReturnsWithin(t, result, 5*time.Second), context.Canceled)
}
