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
	"fmt"
	"io"
	authenticationv1 "k8s.io/api/authentication/v1"
	"net/http"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
)

const kubetokenPath = "auth/kubetoken"

const ErrDynamicKubetokenNotSupported = "using dynamic kubetokens is not supported by your Okteto cluster"

type KubeTokenClient struct {
	httpClient  *http.Client
	url         string
	contextName string
}

type KubeTokenResponse struct {
	authenticationv1.TokenRequest
}

func NewKubeTokenClient(contextName, token, namespace string) (*KubeTokenClient, error) {
	if contextName == "" {
		return nil, oktetoErrors.ErrCtxNotSet
	}

	httpClient, url, err := newOktetoHttpClient(contextName, token, fmt.Sprintf("%s/%s", kubetokenPath, namespace))
	if err != nil {
		return nil, err
	}

	return &KubeTokenClient{
		httpClient:  httpClient,
		url:         url,
		contextName: contextName,
	}, nil
}

func (c *KubeTokenClient) GetKubeToken() (*KubeTokenResponse, error) {
	resp, err := c.httpClient.Get(c.url)
	if err != nil {
		return &KubeTokenResponse{}, fmt.Errorf("failed GET request: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return &KubeTokenResponse{}, fmt.Errorf(oktetoErrors.ErrNotLogged, c.contextName)
	}

	if resp.StatusCode == http.StatusNotFound {
		return &KubeTokenResponse{}, fmt.Errorf(ErrDynamicKubetokenNotSupported)
	}

	if resp.StatusCode != http.StatusOK {
		return &KubeTokenResponse{}, fmt.Errorf("GET request returned status %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &KubeTokenResponse{}, fmt.Errorf("failed to read kubetoken response: %w", err)
	}

	var kubeTokenResponse KubeTokenResponse
	err = json.Unmarshal(body, &kubeTokenResponse)
	if err != nil {
		return &KubeTokenResponse{}, fmt.Errorf("failed to unmarshal kubetoken response: %w", err)
	}

	return &kubeTokenResponse, nil
}

func (k *KubeTokenResponse) ToJson() (string, error) {
	bytes, err := json.MarshalIndent(k, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
