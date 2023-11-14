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

package login

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"

	"github.com/okteto/okteto/pkg/cmd/login/html"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
)

// Handler handles the authentication using a browser
type Handler struct {
	ctx      context.Context
	response chan *types.User
	errChan  chan error
	state    string
	baseURL  string
	port     int
}

func (h *Handler) handle() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		s := r.URL.Query().Get("state")

		if h.state != s {
			h.errChan <- fmt.Errorf("invalid request state")
			return
		}

		ctx := r.Context()
		oktetoClient, err := okteto.NewOktetoClientFromUrl(h.baseURL)
		if err != nil {
			h.errChan <- err
			return
		}
		u, err := oktetoClient.Auth(ctx, code)
		if err != nil {
			if err := html.ExecuteError(w, err); err != nil {
				h.errChan <- err
				return
			}
			// we need to return a legible error for the CLI to display
			h.errChan <- okteto.TranslateAuthError(err)
			return
		}
		if err := html.ExecuteSuccess(w); err != nil {
			h.errChan <- err
			return
		}

		h.response <- u
	}

	return http.HandlerFunc(fn)
}

// AuthorizationURL returns the authorization URL used for login
func (h *Handler) AuthorizationURL() (string, error) {
	redirectURL := fmt.Sprintf("http://127.0.0.1:%d/authorization-code/callback?state=%s", h.port, h.state)
	params := url.Values{}
	params.Add("state", h.state)
	params.Add("redirect", redirectURL)

	authorizationURL, err := url.Parse(fmt.Sprintf("%s/auth/authorization-code", h.baseURL))
	if err != nil {
		return "", fmt.Errorf("failed to build authorizationURL: %w", err)
	}

	authorizationURL.RawQuery = params.Encode()
	return authorizationURL.String(), nil
}

func randToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(b), nil
}
