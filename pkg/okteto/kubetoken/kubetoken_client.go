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
package kubetoken

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	authenticationv1 "k8s.io/api/authentication/v1"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/okteto"
)

const kubetokenPath = "auth/kubetoken"

type cacheSetter interface {
	Set(contextName, namespace string, token authenticationv1.TokenRequest)
}

type Client struct {
	httpClient  *http.Client
	url         string
	contextName string
	namespace   string
	cache       cacheSetter
}

func NewClient(contextName, token, namespace, certificate string, isInsecure bool, cache cacheSetter) (*Client, error) {
	if contextName == "" {
		return nil, oktetoErrors.ErrCtxNotSet
	}

	httpClient, url, err := okteto.NewOktetoHttpClient(contextName, token, fmt.Sprintf("%s/%s", kubetokenPath, namespace), certificate, isInsecure)
	if err != nil {
		return nil, err
	}

	return &Client{
		httpClient:  httpClient,
		url:         url,
		contextName: contextName,
		namespace:   namespace,
		cache:       cache,
	}, nil
}

func (c *Client) GetKubeToken() (string, error) {
	resp, err := c.httpClient.Get(c.url)
	if err != nil {
		return "", fmt.Errorf("failed GET request: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return "", fmt.Errorf(oktetoErrors.ErrNotLogged, c.contextName)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET request returned status %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read kubetoken response: %w", err)
	}

	token := authenticationv1.TokenRequest{}

	if err := json.Unmarshal(body, &token); err != nil {
		return "", fmt.Errorf("failed to unmarshal kubetoken response: %w", err)
	}

	c.cache.Set(c.contextName, c.namespace, token)

	return string(body), nil
}
