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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/types"
)

const (
	catalogItemIDQueryParam = "catalogItemId"

	// catalogSecretVariablesDisabledReason is the reason of the 422 response
	// when the installation has secret variables disabled
	catalogSecretVariablesDisabledReason = "feature_disabled"
)

// Reasons why a secret variable of a catalog item can't be resolved
const (
	CatalogSecretNotFoundReason    = "secret_not_found"
	CatalogSecretNotLabeledReason  = "secret_not_labeled"
	CatalogSecretKeyNotFoundReason = "key_not_found"
)

// ErrCatalogSecretVariablesDisabled is returned when the catalog item has
// secret variables but the installation has them disabled
var ErrCatalogSecretVariablesDisabled = errors.New("secret variables are disabled on this installation")

// CatalogSecretKeyRef is the key of the Secret a secret variable refers to
type CatalogSecretKeyRef struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

// UnresolvedCatalogSecretVariable is a secret variable of a catalog item whose
// Secret or key doesn't exist, or whose Secret isn't labelled to be read by
// catalog items
type UnresolvedCatalogSecretVariable struct {
	Variable     string              `json:"variable"`
	Reason       string              `json:"reason"`
	SecretKeyRef CatalogSecretKeyRef `json:"secretKeyRef"`
}

// UnresolvedCatalogSecretVariablesError is returned when some secret variables
// of a catalog item can't be resolved. It lists all of them
type UnresolvedCatalogSecretVariablesError struct {
	Variables []UnresolvedCatalogSecretVariable
}

func (UnresolvedCatalogSecretVariablesError) Error() string {
	return "the secret variables of the catalog item could not be resolved"
}

type catalogSecretVariablesResponse struct {
	Variables []types.CatalogSecretVariable `json:"variables"`
}

type catalogSecretVariablesErrorResponse struct {
	Reason string                            `json:"reason"`
	Errors []UnresolvedCatalogSecretVariable `json:"errors"`
}

// GetCatalogSecretVariables returns the values of the secret variables of a
// catalog item. The endpoint only answers requests that come from BuildKit
func (c *userClient) GetCatalogSecretVariables(ctx context.Context, endpoint, catalogItemID string) ([]types.CatalogSecretVariable, error) {
	if c.httpClient == nil {
		return nil, fmt.Errorf("catalog secret variables client not configured")
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid catalog secret variables endpoint: %w", err)
	}
	query := u.Query()
	query.Set(catalogItemIDQueryParam, catalogItemID)
	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach the catalog secret variables endpoint: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			oktetoLog.Infof("could not close the body: %s", err)
		}
	}()

	switch resp.StatusCode {
	case http.StatusOK:
		var body catalogSecretVariablesResponse
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			return nil, fmt.Errorf("failed to read the catalog secret variables: %w", err)
		}
		return body.Variables, nil
	case http.StatusNotFound:
		// The catalog item was deleted after deploy or destroy checked it.
		// A deleted item has no secret variables
		oktetoLog.Warning("Catalog item '%s' not found: running the commands without its secret variables", catalogItemID)
		return nil, nil
	case http.StatusUnprocessableEntity:
		var body catalogSecretVariablesErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			oktetoLog.Infof("could not read the catalog secret variables error: %s", err)
			break
		}
		if body.Reason == catalogSecretVariablesDisabledReason {
			return nil, ErrCatalogSecretVariablesDisabled
		}
		if len(body.Errors) > 0 {
			return nil, UnresolvedCatalogSecretVariablesError{Variables: body.Errors}
		}
	}
	return nil, fmt.Errorf("catalog secret variables request failed: %s", resp.Status)
}
