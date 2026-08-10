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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/stretchr/testify/require"
)

const (
	testService          = "api"
	testSharedNamespace  = "staging"
	testBaselineHostname = "api-baseline.staging.svc.cluster.local"
	testBaselineHost     = "api-baseline.staging.svc.cluster.local:80"
	testDivertedHost     = "api.alice-dev.svc.cluster.local:80"
	testServicePort      = 80
	testListenPort       = 8080
	testRoutes           = "alice:alice-dev"
	testInboundHost      = "frontend.staging"
	testRequestURL       = "http://frontend.staging/orders?page=2"
)

func testPort() PortConfig {
	return PortConfig{Name: "http", Listen: testListenPort, Service: testServicePort}
}

// recordingTransport captures the request the router intended to send and then redirects it
// to a local test server, since in-cluster addresses do not resolve from a unit test.
type recordingTransport struct {
	base     http.RoundTripper
	upstream string
	requests []*http.Request
}

func (t *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.requests = append(t.requests, req.Clone(req.Context()))

	out := req.Clone(req.Context())
	out.URL.Host = t.upstream
	out.RequestURI = ""

	return t.base.RoundTrip(out)
}

// failingTransport simulates an unreachable destination.
type failingTransport struct {
	err error
}

func (t *failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, t.err
}

func testConfig() *Config {
	return &Config{
		Service:         testService,
		SharedNamespace: testSharedNamespace,
		BaselineHost:    testBaselineHostname,
		Ports:           []PortConfig{testPort()},
		MaxHops:         defaultMaxHops,
	}
}

func newTestRouter(t *testing.T) (*Router, *recordingTransport) {
	t.Helper()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(upstream.Close)

	transport := &recordingTransport{
		base:     http.DefaultTransport,
		upstream: strings.TrimPrefix(upstream.URL, "http://"),
	}

	r := New(testConfig(), testPort(), NewStaticTable(testService, testRoutes, &testLogger{}), &testLogger{})
	r.proxy.Transport = transport

	return r, transport
}

// Each port dials its own upstream port: a request arriving on the gRPC port must not be
// forwarded to the HTTP one.
func TestNew_EachPortDialsItsOwnUpstreamPort(t *testing.T) {
	cfg := testConfig()
	cfg.Ports = append(cfg.Ports, PortConfig{Name: "grpc", Listen: 9090, Service: 9091})

	http1 := New(cfg, cfg.Ports[0], NewStaticTable(testService, testRoutes, &testLogger{}), &testLogger{})
	grpc := New(cfg, cfg.Ports[1], NewStaticTable(testService, testRoutes, &testLogger{}), &testLogger{})

	require.Equal(t, "api-baseline.staging.svc.cluster.local:80", http1.baselineHost)
	require.Equal(t, "api-baseline.staging.svc.cluster.local:9091", grpc.baselineHost)
	require.Equal(t, "api.alice-dev.svc.cluster.local:9091", grpc.resolve(newDivertRequest("alice")).target)
}

func newDivertRequest(key string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, testRequestURL, nil)
	req.Header.Set(constants.OktetoDivertBaggageHeader, "divert="+key)
	return req
}

func TestRouterServeHTTP_WithoutHeaderGoesToBaseline(t *testing.T) {
	r, transport := newTestRouter(t)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, testRequestURL, nil))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, transport.requests, 1)
	require.Equal(t, testBaselineHost, transport.requests[0].URL.Host)
}

func TestRouterServeHTTP_UnknownKeyGoesToBaseline(t *testing.T) {
	r, transport := newTestRouter(t)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, newDivertRequest("carol"))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, transport.requests, 1)
	require.Equal(t, testBaselineHost, transport.requests[0].URL.Host)
}

func TestRouterServeHTTP_KnownKeyGoesToTheDeveloperNamespace(t *testing.T) {
	r, transport := newTestRouter(t)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, newDivertRequest("alice"))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, transport.requests, 1)
	require.Equal(t, testDivertedHost, transport.requests[0].URL.Host)
}

func TestRouterServeHTTP_PreservesTheHostHeader(t *testing.T) {
	r, transport := newTestRouter(t)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, newDivertRequest("alice"))

	require.Len(t, transport.requests, 1)
	require.Equal(t, testInboundHost, transport.requests[0].Host)
}

func TestRouterServeHTTP_PreservesThePathAndQuery(t *testing.T) {
	r, transport := newTestRouter(t)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, newDivertRequest("alice"))

	require.Len(t, transport.requests, 1)
	require.Equal(t, "/orders", transport.requests[0].URL.Path)
	require.Equal(t, "page=2", transport.requests[0].URL.RawQuery)
}

func TestRouterServeHTTP_StartsTheHopCounter(t *testing.T) {
	r, transport := newTestRouter(t)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, newDivertRequest("alice"))

	require.Len(t, transport.requests, 1)
	require.Equal(t, "1", transport.requests[0].Header.Get(hopsHeader))
}

func TestRouterServeHTTP_IncrementsTheHopCounter(t *testing.T) {
	r, transport := newTestRouter(t)
	req := newDivertRequest("alice")
	req.Header.Set(hopsHeader, "2")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	require.Len(t, transport.requests, 1)
	require.Equal(t, "3", transport.requests[0].Header.Get(hopsHeader))
}

func TestRouterServeHTTP_RejectsALoop(t *testing.T) {
	r, transport := newTestRouter(t)
	req := newDivertRequest("alice")
	req.Header.Set(hopsHeader, "5")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusLoopDetected, rec.Code)
	require.Empty(t, transport.requests)
}

func TestRouterServeHTTP_UnreachableDestinationReturnsBadGateway(t *testing.T) {
	r, _ := newTestRouter(t)
	r.proxy.Transport = &failingTransport{err: errors.New("connection refused")}
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, newDivertRequest("alice"))

	require.Equal(t, http.StatusBadGateway, rec.Code)
}

func TestRouterServeHTTP_JoinsMultipleBaggageHeaders(t *testing.T) {
	r, transport := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, testRequestURL, nil)
	req.Header.Add(constants.OktetoDivertBaggageHeader, "userId=42")
	req.Header.Add(constants.OktetoDivertBaggageHeader, "divert=alice")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	require.Len(t, transport.requests, 1)
	require.Equal(t, testDivertedHost, transport.requests[0].URL.Host)
}

func TestRouterResolve(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected decision
	}{
		{
			name:     "no baggage header",
			header:   "",
			expected: decision{target: testBaselineHost},
		},
		{
			name:     "baggage without a divert key",
			header:   "userId=42,sessionId=abc",
			expected: decision{target: testBaselineHost},
		},
		{
			name:     "known routing key",
			header:   "divert=alice",
			expected: decision{target: testDivertedHost, key: "alice", diverted: true},
		},
		{
			name:     "stale routing key falls back to baseline",
			header:   "divert=carol",
			expected: decision{target: testBaselineHost, key: "carol"},
		},
		{
			name:     "routing key among other members",
			header:   "userId=42,divert=alice;ttl=60",
			expected: decision{target: testDivertedHost, key: "alice", diverted: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, _ := newTestRouter(t)
			req := httptest.NewRequest(http.MethodGet, testRequestURL, nil)
			req.Header.Set(constants.OktetoDivertBaggageHeader, tt.header)

			require.Equal(t, tt.expected, r.resolve(req))
		})
	}
}

func TestCurrentHops(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected int
	}{
		{name: "no header", header: "", expected: 0},
		{name: "valid count", header: "3", expected: 3},
		{name: "not a number", header: "many", expected: 0},
		{name: "negative", header: "-1", expected: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, testRequestURL, nil)
			req.Header.Set(hopsHeader, tt.header)

			require.Equal(t, tt.expected, currentHops(req))
		})
	}
}
