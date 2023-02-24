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
	"context"
	"fmt"
	"io"
	"net/http"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoHttp "github.com/okteto/okteto/pkg/http"
	"golang.org/x/oauth2"
)

const kubetokenPath = "auth/kubetoken"

type kubeTokenClient struct {
	httpClient *http.Client
	url        string
}

func NewKubeTokenClient() (*kubeTokenClient, error) {
	token := Context().Token
	if token == "" {
		return nil, fmt.Errorf(oktetoErrors.ErrNotLogged, Context().Name)
	}
	u := Context().Name
	if u == "" {
		return nil, fmt.Errorf("the okteto URL is not set")
	}

	parsed, err := parseOktetoURLWithPath(u, kubetokenPath)
	if err != nil {
		return nil, err
	}

	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token,
			TokenType: "Bearer"},
	)

	ctxHttpClient := http.DefaultClient

	if insecureSkipTLSVerify {
		ctxHttpClient = oktetoHttp.InsecureHTTPClient()
	} else if cert, err := GetContextCertificate(); err == nil {
		ctxHttpClient = oktetoHttp.StrictSSLHTTPClient(cert)
	}

	ctx := contextWithOauth2HttpClient(context.Background(), ctxHttpClient)

	httpClient := oauth2.NewClient(ctx, src)

	return &kubeTokenClient{
		httpClient: httpClient,
		url:        parsed,
	}, nil
}

func (c *kubeTokenClient) GetKubeToken() (string, error) {
	resp, err := c.httpClient.Get(c.url)
	if err != nil {
		return "", fmt.Errorf("failed GET request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET request returned status %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read kubetoken response: %w", err)
	}

	return string(body), nil
}
