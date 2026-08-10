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
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/okteto/okteto/pkg/divert/router"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

const (
	// readHeaderTimeout bounds how long a caller may take to send its headers. It is the
	// one timeout a proxy can enforce without also capping legitimate traffic.
	readHeaderTimeout = 10 * time.Second

	// idleTimeout closes keep-alive connections that have gone quiet.
	idleTimeout = 120 * time.Second

	// shutdownTimeout is how long in-flight requests have to finish once a shutdown has
	// started. It must stay below the pod's terminationGracePeriodSeconds, or the kubelet
	// will send SIGKILL while requests are still draining.
	shutdownTimeout = 15 * time.Second

	// healthPath is served on its own listener, never on the proxy port.
	healthPath = "/healthz"
)

// boundHandler is a handler with the listener it has been given.
type boundHandler struct {
	listener net.Listener
	handler  http.Handler
	label    string
}

// serve binds every port the Service exposes plus the readiness port, then serves until the
// context is cancelled or the process is signalled.
//
// Binding happens before anything is served so that a port conflict is reported as a
// startup failure rather than surfacing asynchronously once traffic is already pointed here.
func serve(ctx context.Context, cfg *router.Config, handlers map[int]http.Handler, logger router.Logger) error {
	bound := make([]boundHandler, 0, len(handlers)+1)

	closeAll := func() {
		for _, b := range bound {
			b.listener.Close()
		}
	}

	for _, port := range cfg.Ports {
		listener, err := net.Listen("tcp", listenAddr(port.Listen))
		if err != nil {
			closeAll()
			return fmt.Errorf("error listening on proxy port %d: %w", port.Listen, err)
		}
		bound = append(bound, boundHandler{
			listener: listener,
			handler:  handlers[port.Listen],
			label:    fmt.Sprintf("proxy port %d", port.Listen),
		})
	}

	healthListener, err := net.Listen("tcp", listenAddr(cfg.HealthPort))
	if err != nil {
		closeAll()
		return fmt.Errorf("error listening on health port %d: %w", cfg.HealthPort, err)
	}
	bound = append(bound, boundHandler{
		listener: healthListener,
		handler:  healthHandler(),
		label:    fmt.Sprintf("health port %d", cfg.HealthPort),
	})

	return serveOn(ctx, bound, logger)
}

// serveOn runs every listener until one of them fails or the context is cancelled, then
// drains them all.
func serveOn(ctx context.Context, bound []boundHandler, logger router.Logger) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	servers := make([]*http.Server, 0, len(bound))
	// Buffered, so every goroutine can deliver its result and exit even though only the
	// first one is read.
	errCh := make(chan error, len(bound))

	for _, b := range bound {
		server := newServer(b.handler)
		servers = append(servers, server)

		listener := b.listener
		go func() { errCh <- server.Serve(listener) }()
	}

	var err error
	select {
	case err = <-errCh:
		// Any listener failing brings the rest down with it. A router serving some of its
		// ports but not others, or serving traffic while failing readiness, is worse than a
		// router that restarts.
		logger.Infof("divert router listener stopped: %s", err)
	case <-ctx.Done():
		logger.Infof("divert router received a shutdown signal, draining in-flight requests")
	}

	drain(servers, logger)

	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// newServer builds a server suitable for proxying.
//
// The handler is wrapped for h2c so that cleartext HTTP/2 — which is what gRPC speaks
// inside a cluster — is served rather than silently downgraded to HTTP/1.1. The wrapper
// leaves HTTP/1.1 requests, including `Connection: Upgrade` handshakes, untouched.
//
// ReadTimeout and WriteTimeout are deliberately unset: they apply to the whole request and
// response, so any value would cap streaming responses, server-sent events and upgraded
// connections that the proxy is supposed to pass through untouched.
func newServer(handler http.Handler) *http.Server {
	return &http.Server{
		Handler:           h2c.NewHandler(handler, &http2.Server{}),
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
	}
}

func healthHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(healthPath, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return mux
}

// drain shuts every server down, readiness first.
//
// The readiness listener is the last one bound, so draining in reverse order stops it
// first: the endpoints controller starts pulling this pod out while the proxy ports are
// still finishing the requests they already accepted.
func drain(servers []*http.Server, logger router.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	for i := len(servers) - 1; i >= 0; i-- {
		if err := servers[i].Shutdown(ctx); err != nil {
			logger.Infof("error shutting down a divert router listener: %s", err)
		}
	}
}

func listenAddr(port int) string {
	return net.JoinHostPort("", strconv.Itoa(port))
}
