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

package okteto

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoHttp "github.com/okteto/okteto/pkg/http"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/types"
	"golang.org/x/oauth2"
)

const (
	catalogActionQueryParam = "action"

	// catalogSecretVariablesDisabledReason is the reason of the 422 response
	// when the installation has secret variables disabled
	catalogSecretVariablesDisabledReason = "feature_disabled"
)

// ErrCatalogSecretVariablesDisabled is returned when the development
// environment has secret variables but the installation has them disabled
var ErrCatalogSecretVariablesDisabled = errors.New("secret variables are disabled on this installation")

type catalogSecretVariablesErrorResponse struct {
	Reason string `json:"reason"`
}

// CatalogSecretVariablesClient gets the catalog secret variables from the
// internal API of Okteto
type CatalogSecretVariablesClient struct {
	httpClient *http.Client
}

// NewCatalogSecretVariablesClient returns the client that gets the catalog
// secret variables from endpoint with the token of the current context, or the
// one of the options. An https endpoint is reached through internalAPIIP, as
// NewInternalAPIHTTPClient does, and fails without a valid one. Older
// installations give an http endpoint on the API service, which is resolved as
// before
func NewCatalogSecretVariablesClient(endpoint, internalAPIIP string, opts ...Option) (*CatalogSecretVariablesClient, error) {
	cfg := defaultOktetoClientCfg()
	for _, opt := range opts {
		opt(cfg)
	}
	if cfg.Token == "" {
		return nil, fmt.Errorf(oktetoErrors.ErrNotLogged, cfg.CtxName)
	}
	var certs []*x509.Certificate
	if cert, err := GetContextCertificate(); err == nil {
		certs = append(certs, cert)
	}

	httpClient, err := catalogSecretVariablesHTTPClient(endpoint, internalAPIIP, certs)
	if err != nil {
		return nil, err
	}
	httpClient.Transport = &VersionedTransport{
		Version: config.VersionString,
		RoundTripper: &oauth2.Transport{
			Source: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: cfg.Token, TokenType: "Bearer"}),
			Base:   httpClient.Transport,
		},
	}
	return &CatalogSecretVariablesClient{httpClient: httpClient}, nil
}

func catalogSecretVariablesHTTPClient(endpoint, internalAPIIP string, certs []*x509.Certificate) (*http.Client, error) {
	if u, err := url.Parse(endpoint); err == nil && u.Scheme == "http" {
		return oktetoHttp.StrictSSLHTTPClient(&oktetoHttp.SSLTransportOption{Certs: certs}), nil
	}
	return NewInternalAPIHTTPClient(endpoint, internalAPIIP, certs...)
}

// GetCatalogSecretVariables returns the catalog secret variables of the
// development environment the action runs for: the values of the ones that
// can be resolved, and the ones that can't. The endpoint only answers requests
// that come from BuildKit
func (c *CatalogSecretVariablesClient) GetCatalogSecretVariables(ctx context.Context, endpoint, action string) (types.CatalogSecretVariables, error) {
	if c.httpClient == nil {
		return types.CatalogSecretVariables{}, fmt.Errorf("catalog secret variables client not configured")
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return types.CatalogSecretVariables{}, fmt.Errorf("invalid catalog secret variables endpoint: %w", err)
	}
	query := u.Query()
	query.Set(catalogActionQueryParam, action)
	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return types.CatalogSecretVariables{}, err
	}

	// The token is only for the endpoint, so a redirect isn't followed: the
	// oauth2 transport would send it to the next host too
	httpClient := *c.httpClient
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return types.CatalogSecretVariables{}, fmt.Errorf("could not reach the catalog secret variables endpoint: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			oktetoLog.Infof("could not close the body: %s", err)
		}
	}()

	switch resp.StatusCode {
	case http.StatusOK:
		var body types.CatalogSecretVariables
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			return types.CatalogSecretVariables{}, fmt.Errorf("failed to read the catalog secret variables: %w", err)
		}
		return body, nil
	case http.StatusUnprocessableEntity:
		var body catalogSecretVariablesErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			oktetoLog.Infof("could not read the catalog secret variables error: %s", err)
			break
		}
		if body.Reason == catalogSecretVariablesDisabledReason {
			return types.CatalogSecretVariables{}, ErrCatalogSecretVariablesDisabled
		}
	}
	return types.CatalogSecretVariables{}, fmt.Errorf("catalog secret variables request failed: %s", resp.Status)
}
