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

package divertrouter

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/okteto/okteto/pkg/divert/router"
	"github.com/stretchr/testify/require"
)

// testLogger records everything written to it.
type testLogger struct {
	messages []string
}

func (l *testLogger) Infof(format string, args ...interface{}) {
	l.messages = append(l.messages, fmt.Sprintf(format, args...))
}

// okHandler stands in for the router on the proxy listener.
func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func mustListen(t *testing.T) net.Listener {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	return listener
}

// mustListenAnyInterface binds the wildcard address, the same way the router does. A
// listener bound only to the loopback address does not reliably conflict with a later
// wildcard bind on BSD-derived systems, which would make the port-conflict tests pass
// vacuously while the server underneath them started for real.
func mustListenAnyInterface(t *testing.T) net.Listener {
	t.Helper()

	listener, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	return listener
}

func portOf(listener net.Listener) int {
	return listener.Addr().(*net.TCPAddr).Port
}

// cancelledContext makes the port-conflict tests fail fast rather than hang, should a bind
// unexpectedly succeed.
func cancelledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func getStatus(t *testing.T, url string) int {
	t.Helper()

	resp, err := http.Get(url)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	return resp.StatusCode
}

func TestHealthHandler_ReportsReady(t *testing.T) {
	rec := httptest.NewRecorder()

	healthHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, healthPath, nil))

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestHealthHandler_DoesNotAnswerOtherPaths(t *testing.T) {
	rec := httptest.NewRecorder()

	healthHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/orders", nil))

	require.Equal(t, http.StatusNotFound, rec.Code)
}

// A proxy cannot enforce whole-request timeouts without truncating the streaming responses
// it is meant to pass through, so only the header and idle timeouts may be set.
func TestNewServer_DoesNotCapWholeRequestDurations(t *testing.T) {
	server := newServer(okHandler())

	require.Equal(t, readHeaderTimeout, server.ReadHeaderTimeout)
	require.Equal(t, idleTimeout, server.IdleTimeout)
	require.Zero(t, server.ReadTimeout)
	require.Zero(t, server.WriteTimeout)
}

func TestListenAddr_BindsEveryInterface(t *testing.T) {
	require.Equal(t, ":8080", listenAddr(8080))
}

// bind pairs listeners with handlers the way serve does, health last.
func bind(listeners ...net.Listener) []boundHandler {
	bound := make([]boundHandler, 0, len(listeners))
	for i, listener := range listeners {
		handler := okHandler()
		if i == len(listeners)-1 {
			handler = healthHandler()
		}
		bound = append(bound, boundHandler{listener: listener, handler: handler})
	}

	return bound
}

func TestServeOn_ServesBothListenersUntilTheContextIsCancelled(t *testing.T) {
	proxyListener := mustListen(t)
	healthListener := mustListen(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() { done <- serveOn(ctx, bind(proxyListener, healthListener), &testLogger{}) }()

	require.Equal(t, http.StatusOK, getStatus(t, fmt.Sprintf("http://%s%s", healthListener.Addr(), healthPath)))
	require.Equal(t, http.StatusOK, getStatus(t, fmt.Sprintf("http://%s/orders", proxyListener.Addr())))

	cancel()

	require.NoError(t, <-done)
}

// A Service with two ports needs both of them served; one of them missing is a divert that
// works for some traffic and silently drops the rest.
func TestServeOn_ServesEveryProxiedPort(t *testing.T) {
	first := mustListen(t)
	second := mustListen(t)
	healthListener := mustListen(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() { done <- serveOn(ctx, bind(first, second, healthListener), &testLogger{}) }()

	require.Equal(t, http.StatusOK, getStatus(t, fmt.Sprintf("http://%s/orders", first.Addr())))
	require.Equal(t, http.StatusOK, getStatus(t, fmt.Sprintf("http://%s/orders", second.Addr())))

	cancel()

	require.NoError(t, <-done)
}

func TestServeOn_StopsWhenAListenerFails(t *testing.T) {
	proxyListener := mustListen(t)
	healthListener := mustListen(t)
	require.NoError(t, proxyListener.Close())

	err := serveOn(context.Background(), bind(proxyListener, healthListener), &testLogger{})

	require.Error(t, err)
}

func handlersFor(cfg *router.Config) map[int]http.Handler {
	handlers := make(map[int]http.Handler, len(cfg.Ports))
	for _, port := range cfg.Ports {
		handlers[port.Listen] = okHandler()
	}

	return handlers
}

func TestServe_FailsWhenTheProxyPortIsTaken(t *testing.T) {
	occupied := mustListenAnyInterface(t)
	cfg := &router.Config{
		Ports:      []router.PortConfig{{Listen: portOf(occupied), Service: 80}},
		HealthPort: freePort(t),
	}

	err := serve(cancelledContext(), cfg, handlersFor(cfg), &testLogger{})

	require.ErrorContains(t, err, "error listening on proxy port")
}

// The second port being taken must not leave the first one bound.
func TestServe_FailsWhenASecondProxyPortIsTaken(t *testing.T) {
	occupied := mustListenAnyInterface(t)
	cfg := &router.Config{
		Ports: []router.PortConfig{
			{Listen: freePort(t), Service: 80},
			{Listen: portOf(occupied), Service: 90},
		},
		HealthPort: freePort(t),
	}

	err := serve(cancelledContext(), cfg, handlersFor(cfg), &testLogger{})

	require.ErrorContains(t, err, "error listening on proxy port")
	require.NoError(t, canBind(cfg.Ports[0].Listen), "the first port was left bound after the failure")
}

func TestServe_FailsWhenTheHealthPortIsTaken(t *testing.T) {
	occupied := mustListenAnyInterface(t)
	cfg := &router.Config{
		Ports:      []router.PortConfig{{Listen: freePort(t), Service: 80}},
		HealthPort: portOf(occupied),
	}

	err := serve(cancelledContext(), cfg, handlersFor(cfg), &testLogger{})

	require.ErrorContains(t, err, "error listening on health port")
}

// canBind reports whether a port was released.
func canBind(port int) error {
	listener, err := net.Listen("tcp", listenAddr(port))
	if err != nil {
		return err
	}

	return listener.Close()
}
