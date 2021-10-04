// Copyright 2021 The Okteto Authors
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
	"net/url"
	"strings"
	"time"

	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/skratchdot/open-golang/open"
)

func AuthenticateToOktetoCluster(ctx context.Context, oktetoURL, token string) (*okteto.User, error) {
	var user *okteto.User
	var err error
	if len(token) > 0 {
		log.Infof("authenticating with an api token")
		user, err = WithToken(ctx, oktetoURL, token)
		if err != nil {
			return nil, err
		}
	} else if okteto.HasBeenLogged(oktetoURL) {
		log.Infof("re-authenticating with saved token")
		token = okteto.ContextStore().Contexts[oktetoURL].Token
		user, err = WithToken(ctx, oktetoURL, token)
		if err != nil {
			log.Infof("saved token is wrong. Authenticating with browser code")
			user, err = WithBrowser(ctx, oktetoURL)
			if err != nil {
				return nil, err
			}
		}
	} else {
		log.Infof("authenticating with browser code")
		user, err = WithBrowser(ctx, oktetoURL)
		if err != nil {
			return nil, err
		}
	}

	if user.New {
		analytics.TrackSignup(true, user.ID)
	}
	log.Infof("authenticated user %s", user.ID)

	if oktetoURL == okteto.CloudURL {
		log.Success("Logged in as %s", user.ExternalID)
	} else {
		log.Success("Logged in as %s @ %s", user.ExternalID, oktetoURL)
	}

	return user, nil
}

// WithToken authenticates the user with an API token
func WithToken(ctx context.Context, url, token string) (*okteto.User, error) {
	return okteto.AuthWithToken(ctx, url, token)
}

// WithBrowser authenticates the user with the browser
func WithBrowser(ctx context.Context, oktetoURL string) (*okteto.User, error) {
	h, err := StartWithBrowser(ctx, oktetoURL)
	if err != nil {
		log.Infof("couldn't start the login process: %s", err)
		return nil, fmt.Errorf("couldn't start the login process, please try again")
	}

	authorizationURL := h.AuthorizationURL()
	fmt.Println("Authentication will continue in your default browser")
	if err := open.Start(authorizationURL); err != nil {
		if strings.Contains(err.Error(), "executable file not found in $PATH") {
			return nil, errors.UserError{
				E:    fmt.Errorf("No browser could be found"),
				Hint: "Use the '--token' flag to run this command in server mode. More information can be found here: https://okteto.com/docs/reference/cli/#login",
			}
		}
		log.Errorf("Something went wrong opening your browser: %s\n", err)
	}

	fmt.Printf("You can also open a browser and navigate to the following address:\n")
	fmt.Println(authorizationURL)

	return EndWithBrowser(ctx, h)
}

// StartWithBrowser starts the authentication of the user with the IDP via a browser
func StartWithBrowser(ctx context.Context, u string) (*Handler, error) {
	state, err := randToken()
	if err != nil {
		log.Infof("couldn't generate random token: %s", err)
		return nil, fmt.Errorf("couldn't generate a random token, please try again")
	}

	port, err := model.GetAvailablePort(model.Localhost)

	if err != nil {
		log.Infof("couldn't access the network: %s", err)
		return nil, fmt.Errorf("couldn't access the network")
	}

	url, err := url.Parse(u)
	if err != nil {
		return nil, err
	}
	if url.Scheme == "" {
		url.Scheme = "https"
	}

	handler := &Handler{
		baseURL:  url.String(),
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
