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
	"context"
	"crypto/tls"
	"net"
	"net/http"

	"golang.org/x/net/http2"
)

// transport forwards each request over the protocol it arrived on.
//
// gRPC is HTTP/2 over cleartext, and nothing in the cluster is speaking TLS to the router,
// so those requests need an h2c-capable client. HTTP/1.1 requests must keep using the
// standard transport: it is the only one that can carry a WebSocket or any other
// `Connection: Upgrade` exchange, which HTTP/2 has no equivalent for.
//
// Sending everything over one transport breaks one half or the other, and the failure is
// quiet either way — a silent downgrade that makes gRPC hang, or an upgrade that never
// completes.
type transport struct {
	http1 http.RoundTripper
	http2 http.RoundTripper
}

func newTransport() http.RoundTripper {
	return &transport{
		http1: http.DefaultTransport.(*http.Transport).Clone(),
		http2: &http2.Transport{
			// The router only ever talks to Services inside the cluster, so there is no TLS
			// to negotiate and no ALPN to agree on. AllowHTTP plus a plain dialer is what
			// makes prior-knowledge h2c work.
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, addr)
			},
		},
	}
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.ProtoMajor == 2 {
		return t.http2.RoundTrip(req)
	}

	return t.http1.RoundTrip(req)
}
