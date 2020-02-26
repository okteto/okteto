// Copyright 2020 The Okteto Authors
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
	"fmt"
	"net/http"
	"time"

	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
)

// WithToken authenticates the user with an API token
func WithToken(ctx context.Context, url, token string) (*okteto.User, error) {
	return okteto.AuthWithToken(ctx, url, token)
}

// StartWithBrowser starts the authentication of the user with the IDP via a browser
func StartWithBrowser(ctx context.Context, url string) (*Handler, error) {
	state, err := randToken()
	if err != nil {
		log.Infof("couldn't generate random token: %s", err)
		return nil, fmt.Errorf("couldn't generate a random token, please try again")
	}

	port, err := model.GetAvailablePort()

	if err != nil {
		log.Infof("couldn't access the network: %s", err)
		return nil, fmt.Errorf("couldn't access the network")
	}

	handler := &Handler{
		baseURL:  url,
		port:     port,
		ctx:      context.Background(),
		state:    state,
		errChan:  make(chan error, 2),
		response: make(chan string, 2),
	}

	return handler, nil

}

// EndWithBrowser finishes the browser based auth
func EndWithBrowser(ctx context.Context, h *Handler) (*okteto.User, error) {
	go func() {
		http.Handle("/authorization-code/callback", h.handle())
		h.errChan <- http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", h.port), nil)
	}()

	ticker := time.NewTicker(5 * time.Minute)
	var code string

	select {
	case <-ticker.C:
		h.ctx.Done()
		return nil, fmt.Errorf("authentication timeout")
	case code = <-h.response:
		break
	case e := <-h.errChan:
		h.ctx.Done()
		return nil, e
	}

	return okteto.Auth(ctx, code, h.baseURL)
}
