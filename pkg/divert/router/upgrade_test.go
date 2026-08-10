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

package router

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const upgradeTimeout = 5 * time.Second

// echoUpgradeServer completes a `Connection: Upgrade` handshake and then echoes whatever is
// written to it, standing in for a WebSocket endpoint without pulling in a WebSocket library.
func echoUpgradeServer(t *testing.T) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		conn, buffered, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return
		}
		defer conn.Close()

		_, _ = buffered.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
		_ = buffered.Flush()

		line, err := buffered.ReadString('\n')
		if err != nil {
			return
		}
		_, _ = buffered.WriteString("echo: " + line)
		_ = buffered.Flush()
	}))
	t.Cleanup(server.Close)

	return server
}

// A divert has to carry WebSockets, and an upgrade only survives if the proxy hands the
// connection over rather than treating the 101 as an ordinary response.
func TestRouter_PassesAConnectionUpgradeThrough(t *testing.T) {
	upstream := echoUpgradeServer(t)

	cfg := testConfig()
	r := New(cfg, testPort(), NewStaticTable(testService, testRoutes, &testLogger{}), &testLogger{})
	r.proxy.Transport = &recordingTransport{
		base:     http.DefaultTransport,
		upstream: strings.TrimPrefix(upstream.URL, "http://"),
	}

	front := httptest.NewServer(r)
	t.Cleanup(front.Close)

	conn, err := net.DialTimeout("tcp", strings.TrimPrefix(front.URL, "http://"), upgradeTimeout)
	require.NoError(t, err)
	defer conn.Close()
	require.NoError(t, conn.SetDeadline(time.Now().Add(upgradeTimeout)))

	_, err = fmt.Fprintf(conn, "GET /socket HTTP/1.1\r\nHost: frontend.staging\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
	require.NoError(t, err)

	reader := bufio.NewReader(conn)
	status, err := reader.ReadString('\n')
	require.NoError(t, err)
	require.Contains(t, status, "101 Switching Protocols")

	// Drain the rest of the handshake headers.
	for {
		line, err := reader.ReadString('\n')
		require.NoError(t, err)
		if strings.TrimSpace(line) == "" {
			break
		}
	}

	// The connection is now raw: bytes have to flow both ways over it.
	_, err = conn.Write([]byte("ping\n"))
	require.NoError(t, err)

	echoed, err := reader.ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, "echo: ping\n", echoed)
}
