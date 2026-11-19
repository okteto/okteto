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
	errEndpointNotSet  = fmt.Errorf("%s is not set", constants.OktetoGitCredentialsEndpointEnvVar)
	errInvalidEndpoint = fmt.Errorf("%s is not an https URL", constants.OktetoGitCredentialsEndpointEnvVar)
	errTokenNotSet     = fmt.Errorf("%s is not set", model.OktetoTokenEnvVar)
)

// endpointNotMappedError is returned when the endpoint's hostname isn't the one Okteto maps to
// the internal IP of its API. Public DNS resolves that hostname to the ingress, where any user
// can serve it, so the token is never sent there
type endpointNotMappedError struct {
	hostname string
}

func (e endpointNotMappedError) Error() string {
	return fmt.Sprintf("%s isn't mapped to the internal IP of the Okteto API", e.hostname)
}

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

// newGetterFromEnv returns the client for the endpoint in OKTETO_GIT_CREDENTIALS_ENDPOINT and the
// token in OKTETO_TOKEN. The endpoint only answers inside the cluster, so it doesn't load an
// Okteto context.
func newGetterFromEnv() (credentialsGetter, error) {
	return newGetter(
		os.Getenv(constants.OktetoGitCredentialsEndpointEnvVar),
		os.Getenv(constants.OktetoInternalAPIHostnameEnvVar),
		os.Getenv(model.OktetoTokenEnvVar),
		&http.Client{},
	)
}

// newGetter only accepts an https endpoint on internalAPIHostname, the hostname that Okteto's
// installer and remote runs map to the internal IP of the API
func newGetter(endpoint, internalAPIHostname, token string, httpClient *http.Client) (credentialsGetter, error) {
	if endpoint == "" {
		return nil, errEndpointNotSet
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" {
		return nil, errInvalidEndpoint
	}
	if internalAPIHostname == "" || u.Hostname() != internalAPIHostname {
		return nil, endpointNotMappedError{hostname: u.Hostname()}
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
