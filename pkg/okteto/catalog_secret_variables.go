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

// GetCatalogSecretVariables returns the catalog secret variables of the
// development environment the action runs for: the values of the ones that
// can be resolved, and the ones that can't. The endpoint only answers requests
// that come from BuildKit
func (c *userClient) GetCatalogSecretVariables(ctx context.Context, endpoint, action string) (types.CatalogSecretVariables, error) {
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

	resp, err := c.httpClient.Do(req)
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
