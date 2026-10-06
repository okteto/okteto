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

package forward

import (
	"net/http"
	"net/url"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/streaming/pkg/httpstream"
)

// NewDialer returns the dialer used to port-forward to the pod subresource at url.
// Same as kubectl, it first tunnels SPDY over WebSockets and falls back to plain SPDY when
// the API server (or a proxy in between) doesn't support the WebSocket upgrade.
// KUBECTL_PORT_FORWARD_WEBSOCKETS=false disables the WebSocket dialer, as it does for kubectl.
func NewDialer(restConfig *rest.Config, url *url.URL) (httpstream.Dialer, error) {
	transport, upgrader, err := spdy.RoundTripperFor(restConfig)
	if err != nil {
		return nil, err
	}
	spdyDialer := spdy.NewDialerForStreaming(upgrader, &http.Client{Transport: transport}, http.MethodPost, url)

	if cmdutil.PortForwardWebsockets.IsDisabled() {
		return spdyDialer, nil
	}

	websocketDialer, err := portforward.NewSPDYOverWebsocketDialerForStreaming(url, restConfig)
	if err != nil {
		return nil, err
	}
	return portforward.NewFallbackDialerForStreaming(websocketDialer, spdyDialer, shouldFallbackToSPDY), nil
}

func shouldFallbackToSPDY(err error) bool {
	return httpstream.IsUpgradeFailure(err) || httpstream.IsHTTPSProxyError(err)
}
