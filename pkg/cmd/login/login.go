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
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/okteto/okteto/pkg/analytics"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/types"
	"github.com/skratchdot/open-golang/open"
)

type Interface interface {
	AuthenticateToOktetoCluster(context.Context, string, string) (*types.User, error)
}

type Controller struct {
}

func NewLoginController() *Controller {
	return &Controller{}
}

func (*Controller) AuthenticateToOktetoCluster(ctx context.Context, oktetoURL, token string) (*types.User, error) {
	if token == "" {
		oktetoLog.Infof("authenticating with browser code")
		user, err := WithBrowser(ctx, oktetoURL)
		// If there is a TLS error, return the raw error
		if oktetoErrors.IsX509(err) {
			return nil, oktetoErrors.UserError{
				E:    err,
				Hint: oktetoErrors.ErrX509Hint,
			}
		}
		if err != nil {
			return nil, oktetoErrors.UserError{
				E:    fmt.Errorf("couldn't authenticate to okteto context: %w", err),
				Hint: "Try to set the context using the 'token' flag: https://www.okteto.com/docs/reference/cli/#context",
			}
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
		return nil, fmt.Errorf("couldn't start the login process: %w", err)
	}

	authorizationURL, err := h.AuthorizationURL()
	if err != nil {
		return nil, err
	}
	oktetoLog.Println("Authentication will continue in your default browser")
	if err := open.Start(authorizationURL); err != nil {
		if strings.Contains(err.Error(), "executable file not found in $PATH") {
			return nil, oktetoErrors.UserError{
				E:    fmt.Errorf("no browser could be found"),
				Hint: "Use the '--token' flag to run this command in server mode. More information can be found here: https://www.okteto.com/docs/reference/cli/#context",
			}
		}
		oktetoLog.Errorf("Something went wrong opening your browser: %s\n", err)
	}

	oktetoLog.Printf("You can also open a browser and navigate to the following address:\n")
	oktetoLog.Println(authorizationURL)

	return EndWithBrowser(h)
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
		ctx:      ctx,
		state:    state,
		errChan:  make(chan error, 2),
		response: make(chan *types.User, 2),
	}

	return handler, nil

}

// EndWithBrowser finishes the browser based auth
func EndWithBrowser(h *Handler) (*types.User, error) {
	go func() {
		sm := http.NewServeMux()
		sm.Handle("/authorization-code/callback", h.handle())
		server := &http.Server{
			Addr:              net.JoinHostPort("127.0.0.1", strconv.Itoa(h.port)),
			Handler:           sm,
			ReadHeaderTimeout: 3 * time.Second,
		}

		h.errChan <- server.ListenAndServe()
	}()

	ticker := time.NewTicker(5 * time.Minute)
	var user *types.User

	select {
	case <-ticker.C:
		h.ctx.Done()
		return nil, fmt.Errorf("authentication timeout")
	case user = <-h.response:
		break
	case e := <-h.errChan:
		h.ctx.Done()
		return nil, e
	}

	return user, nil
}
