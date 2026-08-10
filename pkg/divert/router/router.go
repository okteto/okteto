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

// Package router implements the data plane of the mesh-free header-based divert: a reverse
// proxy that sits in front of a service in the shared namespace and, per request, forwards
// either to the untouched baseline or to a developer's copy of that same service.
//
// The package is deliberately free of Kubernetes and cobra dependencies so that it can be
// exercised in unit tests and reused by any entry point that wants to serve it.
package router

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"
	"time"

	"github.com/okteto/okteto/pkg/constants"
)

const (
	// divertBaggageKey is the W3C Baggage member carrying the routing key, as in
	// `baggage: divert=alice`.
	divertBaggageKey = "divert"

	// hopsHeader counts how many diverted routers a request has already traversed. It is
	// what catches a diverted service that ends up calling its own name.
	hopsHeader = "X-Divert-Hops"

	// clusterDomain is the in-cluster DNS suffix used to address the developer's copy.
	clusterDomain = "svc.cluster.local"
)

// Logger is the minimal logging surface the router needs. It mirrors the interface used
// elsewhere in pkg/divert so that entry points can supply the logger they already have.
type Logger interface {
	Infof(format string, args ...interface{})
}

// noopLogger discards everything. Used when no logger is supplied so that the router never
// has to nil-check on the request path.
type noopLogger struct{}

func (noopLogger) Infof(string, ...interface{}) {}

// Router forwards requests to the baseline or to a developer namespace based on the
// routing key found in the request's baggage header.
//
// One Router serves one port. A Service exposing several ports gets one per port, each
// dialling its own upstream port, because the port a request arrived on determines which
// port it has to be forwarded to.
type Router struct {
	table           Table
	logger          Logger
	proxy           *httputil.ReverseProxy
	service         string
	baselineHost    string
	destinationPort int
	maxHops         int
}

// decision is the outcome of resolving one request against the route table.
type decision struct {
	target   string
	key      string
	diverted bool
}

// decisionContextKey carries the decision from ServeHTTP to the proxy's Rewrite hook, so a
// single ReverseProxy (and therefore a single pool of upstream connections) can serve every
// destination instead of one being built per request.
type decisionContextKey struct{}

// New builds a Router serving one port of the diverted Service. A nil logger is replaced
// with a no-op.
func New(cfg *Config, port PortConfig, table Table, logger Logger) *Router {
	if logger == nil {
		logger = noopLogger{}
	}

	r := &Router{
		service:         cfg.Service,
		baselineHost:    net.JoinHostPort(cfg.BaselineHost, strconv.Itoa(port.Service)),
		destinationPort: port.Service,
		maxHops:         cfg.MaxHops,
		table:           table,
		logger:          logger,
	}

	r.proxy = &httputil.ReverseProxy{
		Rewrite: r.rewrite,
		// Disable response buffering. Without it, streaming responses and server-sent
		// events are held back until the buffer fills, which looks like a hung application
		// rather than a proxy setting.
		FlushInterval: -1,
		ErrorHandler:  r.handleProxyError,
		Transport:     newTransport(),
	}

	return r
}

// ServeHTTP implements http.Handler.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	hops := currentHops(req)
	if hops >= r.maxHops {
		r.logger.Infof("divert loop detected for service %q: %d hops reached the limit of %d", r.service, hops, r.maxHops)
		http.Error(w, "divert loop detected", http.StatusLoopDetected)
		return
	}

	d := r.resolve(req)

	start := time.Now()
	r.proxy.ServeHTTP(w, req.WithContext(context.WithValue(req.Context(), decisionContextKey{}, d)))

	r.logger.Infof(
		"service=%s key=%q diverted=%t target=%s hops=%d duration=%s",
		r.service, d.key, d.diverted, d.target, hops, time.Since(start),
	)
}

// resolve picks the destination for a request. Every path that is not an explicit,
// currently-registered routing key resolves to the baseline: an unknown or stale key must
// degrade to normal behaviour, never to a 404.
func (r *Router) resolve(req *http.Request) decision {
	// A request may legitimately carry more than one baggage header; the W3C
	// specification defines them as a single comma-separated list.
	header := strings.Join(req.Header.Values(constants.OktetoDivertBaggageHeader), ",")

	key := baggageValue(header, divertBaggageKey)
	if key == "" {
		return decision{target: r.baselineHost}
	}

	namespace, ok := r.table.Lookup(r.service, key)
	if !ok {
		return decision{target: r.baselineHost, key: key}
	}

	host := fmt.Sprintf("%s.%s.%s", r.service, namespace, clusterDomain)
	return decision{
		target:   net.JoinHostPort(host, strconv.Itoa(r.destinationPort)),
		key:      key,
		diverted: true,
	}
}

// rewrite points the outbound request at the resolved destination.
func (r *Router) rewrite(pr *httputil.ProxyRequest) {
	d, _ := pr.In.Context().Value(decisionContextKey{}).(decision)

	pr.Out.URL.Scheme = "http"
	pr.Out.URL.Host = d.target

	// Keep the Host the caller asked for. ReverseProxy would otherwise rewrite it to the
	// upstream address, which breaks applications that route on Host.
	pr.Out.Host = pr.In.Host

	pr.SetXForwarded()
	pr.Out.Header.Set(hopsHeader, strconv.Itoa(currentHops(pr.In)+1))
}

// handleProxyError reports an unreachable destination as a bad gateway. The developer's
// copy being down is the common case, and it must not look like an error in the caller.
func (r *Router) handleProxyError(w http.ResponseWriter, req *http.Request, err error) {
	d, _ := req.Context().Value(decisionContextKey{}).(decision)
	r.logger.Infof("error proxying service %q to %s: %s", r.service, d.target, err)
	w.WriteHeader(http.StatusBadGateway)
}

// currentHops reads the hop counter, treating anything unparseable as a fresh request.
func currentHops(req *http.Request) int {
	hops, err := strconv.Atoi(req.Header.Get(hopsHeader))
	if err != nil || hops < 0 {
		return 0
	}
	return hops
}
