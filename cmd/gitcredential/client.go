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

package gitcredential

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
)

const (
	hostQueryParam       = "host"
	repositoryQueryParam = "repository"

	// maxResponseSize caps what the helper reads from the endpoint, whose answer is a few short fields
	maxResponseSize = 64 << 10
)

var (
	// errEndpointNotSet means the helper runs outside an Okteto installer and its remote runs,
	// where there are no credentials to give
	errEndpointNotSet = fmt.Errorf("%s is not set", constants.OktetoGitCredentialsEndpointEnvVar)
	errTokenNotSet    = fmt.Errorf("%s is not set", model.OktetoTokenEnvVar)
)

// gitCredentials are the credentials Okteto gives for a Git repository.
// Without credentials, Password is empty and Reason says why
type gitCredentials struct {
	// ExpiresAt is zero when the expiry is unknown
	ExpiresAt time.Time
	Username  string
	Password  string
	// Reason is a sentence for the user, set when there are no credentials
	Reason string
}

// credentialsResponse is the body of the endpoint's 200 answer: the credentials, or only the reason
type credentialsResponse struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	ExpiresAt string `json:"expiresAt"`
	Reason    string `json:"reason"`
}

// endpointClient asks the endpoint that Okteto sets in its installer and remote runs for the credentials
type endpointClient struct {
	httpClient *http.Client
	endpoint   *url.URL
	token      string
}

// newGetterFromEnv returns the client for the endpoint in OKTETO_GIT_CREDENTIALS_ENDPOINT, the
// internal IP of the Okteto API in OKTETO_INTERNAL_API_IP and the token in OKTETO_TOKEN. The
// endpoint only answers inside the cluster, so it doesn't load an Okteto context.
func newGetterFromEnv() (credentialsGetter, error) {
	return newGetter(
		os.Getenv(constants.OktetoGitCredentialsEndpointEnvVar),
		os.Getenv(constants.OktetoInternalAPIIPEnvVar),
		os.Getenv(model.OktetoTokenEnvVar),
	)
}

// newGetter only sends the token over https to internalAPIIP, never to the address public DNS gives
// for the endpoint's hostname, which is the ingress, where any user can serve it. certs are trusted
// on top of the system pool
func newGetter(endpoint, internalAPIIP, token string, certs ...*x509.Certificate) (credentialsGetter, error) {
	if endpoint == "" {
		return nil, errEndpointNotSet
	}
	httpClient, err := okteto.NewInternalAPIHTTPClient(endpoint, internalAPIIP, certs...)
	if errors.Is(err, okteto.ErrInvalidInternalAPIIP) {
		return nil, fmt.Errorf("%w (%s)", err, constants.OktetoInternalAPIIPEnvVar)
	}
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	if token == "" {
		return nil, errTokenNotSet
	}
	return &endpointClient{
		httpClient: httpClient,
		endpoint:   u,
		token:      token,
	}, nil
}

// GetGitCredentials returns the credentials Okteto gives for a repository ("owner/repo") of a Git host.
// Without credentials, the endpoint explains why in Reason.
func (c *endpointClient) GetGitCredentials(ctx context.Context, host, repository string) (gitCredentials, error) {
	u := *c.endpoint
	query := u.Query()
	query.Set(hostQueryParam, host)
	query.Set(repositoryQueryParam, repository)
	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return gitCredentials{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return gitCredentials{}, fmt.Errorf("could not reach the Okteto API: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// The endpoint answers its errors without a body
	if resp.StatusCode != http.StatusOK {
		return gitCredentials{}, fmt.Errorf("the Okteto API answered %s", resp.Status)
	}

	var body credentialsResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseSize)).Decode(&body); err != nil {
		return gitCredentials{}, errors.New("could not read the answer of the Okteto API")
	}

	creds := gitCredentials{
		Username: body.Username,
		Password: body.Password,
		Reason:   body.Reason,
	}
	// Git uses the token until it fails when it doesn't know the expiry, so an expiry
	// that can't be read is dropped instead of the token
	if t, err := time.Parse(time.RFC3339, body.ExpiresAt); err == nil {
		creds.ExpiresAt = t
	}
	return creds, nil
}
