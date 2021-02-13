// Copyright 2020 The Okteto Authors
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

package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
)

var (
	roundTripHost  string
	roundTripToken string
)

type refreshCredentials struct {
	rt http.RoundTripper
}

func refreshCredentialsFn(rt http.RoundTripper) http.RoundTripper {
	return &refreshCredentials{
		rt: rt,
	}
}

func (rc *refreshCredentials) RoundTrip(req *http.Request) (*http.Response, error) {
	if roundTripHost != "" {
		req.Host = roundTripHost
		req.URL.Host = roundTripHost
		req.Header["Authorization"] = []string{fmt.Sprintf("Bearer %s", roundTripToken)}
	}
	resp, errOrig := rc.rt.RoundTrip(req)
	if okteto.GetClusterContext() != currentContext {
		return resp, errOrig
	}
	if req.Method != http.MethodGet {
		return resp, errOrig
	}
	if !isCredentialError(resp, errOrig) {
		return resp, errOrig
	}

	log.Infof("refreshing kubernetes credentials...")
	var err error
	var host string
	ctx := context.Background()
	host, roundTripToken, err = okteto.RefreshOktetoKubeconfig(ctx, namespace)
	if err != nil {
		log.Infof("failed to refresh your kubernetes credentials: %s", err.Error())
		return resp, errOrig
	}

	r, _ := url.Parse(host)
	roundTripHost = r.Host
	req.Host = roundTripHost
	req.URL.Host = roundTripHost
	req.Header["Authorization"] = []string{fmt.Sprintf("Bearer %s", roundTripToken)}
	return rc.rt.RoundTrip(req)
}

func isCredentialError(resp *http.Response, err error) bool {
	if resp != nil && resp.StatusCode == 401 {
		return true
	}
	if err != nil && strings.Contains(err.Error(), "i/o timeout") {
		return true
	}
	if err != nil && strings.Contains(err.Error(), "context deadline exceeded") {
		return true
	}
	if err != nil && strings.Contains(err.Error(), "x509") {
		return true
	}
	if err != nil && strings.Contains(err.Error(), "no such host") {
		return true
	}
	return false
}
