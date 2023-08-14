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

package okteto

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/okteto/okteto/pkg/types"
	"io"
	"net/http"
	"net/url"
)

const (
	// kubetokenPathTemplate (baseURL, namespace)
	kubetokenPathTemplate = "%s/auth/kubetoken/%s"
)

var (
	errRequest               = errors.New("failed request")
	errStatus                = errors.New("status error")
	errUnauthorized          = errors.New("unauthorized")
	errKubetokenNotAvailable = errors.New("kubetoken service not found")
)

type kubeTokenClient struct {
	httpClient *http.Client
}

func newKubeTokenClient(httpClient *http.Client) *kubeTokenClient {
	return &kubeTokenClient{
		httpClient: httpClient,
	}
}

func getKubetokenURL(baseURL, namespace string) (*url.URL, error) {
	return url.Parse(fmt.Sprintf(kubetokenPathTemplate, baseURL, namespace))
}

func (c *kubeTokenClient) GetKubeToken(baseURL, namespace string) (types.KubeTokenResponse, error) {
	endpoint, err := getKubetokenURL(baseURL, namespace)
	if err != nil {
		return types.KubeTokenResponse{}, err
	}

	resp, err := c.httpClient.Get(endpoint.String())
	if err != nil {
		return types.KubeTokenResponse{}, fmt.Errorf("GetKubeToken %w: %w", errRequest, err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return types.KubeTokenResponse{}, fmt.Errorf("GetKubeToken %w", errUnauthorized)
	}

	if resp.StatusCode != http.StatusOK {
		return types.KubeTokenResponse{}, fmt.Errorf("GetKubeToken %w: %s", errStatus, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.KubeTokenResponse{}, fmt.Errorf("failed to read kubetoken response: %w", err)
	}

	var kubeTokenResponse types.KubeTokenResponse
	err = json.Unmarshal(body, &kubeTokenResponse)
	if err != nil {
		return types.KubeTokenResponse{}, fmt.Errorf("failed to unmarshal kubetoken response: %w", err)
	}

	return kubeTokenResponse, nil
}

func (c *kubeTokenClient) CheckService(baseURL, namespace string) error {
	endpoint, err := getKubetokenURL(baseURL, namespace)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Head(endpoint.String())
	if err != nil {
		return fmt.Errorf("CheckService %w: %w", errRequest, err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("CheckService %w: %s", errKubetokenNotAvailable, baseURL)
	}

	return nil
}
