// Copyright 2022 The Okteto Authors
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
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/skratchdot/open-golang/open"
)

type LoginInterface interface {
	AuthenticateToOktetoCluster(context.Context, string, string) (*types.User, error)
}

type LoginController struct {
}

func NewLoginController() *LoginController {
	return &LoginController{}
}

func (l LoginController) AuthenticateToOktetoCluster(ctx context.Context, oktetoURL, token string) (*types.User, error) {
	if token == "" {
		oktetoLog.Infof("authenticating with browser code")
		user, err := WithBrowser(ctx, oktetoURL)
		if err != nil {
			return nil, err
		}
		if user.New {
			analytics.TrackSignup(true, user.ID)
		}
		oktetoLog.Infof("authenticated user %s", user.ID)

		return user, nil
	}
	return &types.User{Token: token}, nil
}

// WithBrowser authenticates the user with the browser
func WithBrowser(ctx context.Context, oktetoURL string) (*types.User, error) {
	h, err := StartWithBrowser(ctx, oktetoURL)
	if err != nil {
		oktetoLog.Infof("couldn't start the login process: %s", err)
		return nil, fmt.Errorf("couldn't start the login process, please try again")
	}

	authorizationURL := h.AuthorizationURL()
	oktetoLog.Println("Authentication will continue in your default browser")
	if err := open.Start(authorizationURL); err != nil {
		if strings.Contains(err.Error(), "executable file not found in $PATH") {
			return nil, oktetoErrors.UserError{
				E:    fmt.Errorf("no browser could be found"),
				Hint: "Use the '--token' flag to run this command in server mode. More information can be found here: https://okteto.com/docs/reference/cli/#login",
			}
		}
		oktetoLog.Errorf("Something went wrong opening your browser: %s\n", err)
	}

	oktetoLog.Printf("You can also open a browser and navigate to the following address:\n")
	oktetoLog.Println(authorizationURL)

	return EndWithBrowser(ctx, h)
}

// StartWithBrowser starts the authentication of the user with the IDP via a browser
func StartWithBrowser(ctx context.Context, u string) (*Handler, error) {
	state, err := randToken()
	if err != nil {
		oktetoLog.Infof("couldn't generate random token: %s", err)
		return nil, fmt.Errorf("couldn't generate a random token, please try again")
	}

	port, err := model.GetAvailablePort(model.Localhost)

	if err != nil {
		oktetoLog.Infof("couldn't access the network: %s", err)
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
func EndWithBrowser(ctx context.Context, h *Handler) (*types.User, error) {
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
