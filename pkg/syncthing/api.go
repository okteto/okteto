// Copyright 2023 The Okteto Authors
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

package syncthing

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	oktetoLog "github.com/okteto/okteto/pkg/log"
)

type addAPIKeyTransport struct {
	T http.RoundTripper
}

const (
	APIKeyHeader      = "X-Api-Key"
	APIKeyHeaderValue = "cnd"
)

// RoundTrip implements the http.RoundTripper interface and is used to add the
// desired request headers to http requests.
func (akt *addAPIKeyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add(APIKeyHeader, APIKeyHeaderValue)
	return akt.T.RoundTrip(req)
}

// NewAPIClient returns a new syncthing api client configured to call the syncthing api
func NewAPIClient() *http.Client {
	return &http.Client{
		Timeout:   60 * time.Second,
		Transport: &addAPIKeyTransport{http.DefaultTransport},
	}
}

// APICall calls the syncthing API and returns the parsed json or an error
func (s *Syncthing) APICall(ctx context.Context, url, method string, code int, params map[string]string, local bool, body []byte, readBody bool, maxRetries int) ([]byte, error) {
	retries := 0
	ticker := time.NewTicker(200 * time.Millisecond)
	for {
		select {
		case <-ticker.C:
			result, err := s.callWithRetry(ctx, url, method, code, params, local, body, readBody)
			if err == nil {
				return result, nil
			}

			if strings.Contains(err.Error(), "connection refused") {
				oktetoLog.Infof("syncthing is not ready, retrying local=%t", local)
			} else {
				oktetoLog.Infof("retrying syncthing call[%s] local=%t: %s", url, local, err.Error())
			}

			if retries >= maxRetries {
				return nil, err
			}
			retries++

		case <-ctx.Done():
			oktetoLog.Infof("call to syncthing.APICall %s canceled", url)
			return nil, ctx.Err()
		}
	}
}

func (s *Syncthing) callWithRetry(ctx context.Context, url, method string, code int, params map[string]string, local bool, body []byte, readBody bool) ([]byte, error) {
	var urlPath string
	if local {
		urlPath = path.Join(s.GUIAddress, url)
		s.Client.Timeout = 5 * time.Second
	} else {
		urlPath = path.Join(s.RemoteGUIAddress, url)
		if url == "rest/system/ping" {
			s.Client.Timeout = 5 * time.Second
		}
	}

	req, err := http.NewRequest(method, fmt.Sprintf("http://%s", urlPath), bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize syncthing API request: %w", err)
	}

	req = req.WithContext(ctx)

	q := req.URL.Query()

	for key, value := range params {
		q.Add(key, value)
	}

	req.URL.RawQuery = q.Encode()

	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call syncthing [%s]: %w", url, err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != code {
		return nil, fmt.Errorf("unexpected response from syncthing [%s | %d]: %s", req.URL.String(), resp.StatusCode, string(body))
	}

	if !readBody {
		return nil, nil
	}

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response from syncthing [%s]: %w", url, err)
	}

	return body, nil
}
