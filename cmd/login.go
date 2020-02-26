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

package cmd

import (
	"context"
	"fmt"
	"net/url"

	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/login"
	"github.com/okteto/okteto/pkg/errors"
	k8Client "github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/cobra"
)

//Login starts the login handshake with github and okteto
func Login() *cobra.Command {
	token := ""
	cmd := &cobra.Command{
		Use:   "login [url]",
		Short: "Login with Okteto",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			if k8Client.InCluster() {
				return errors.ErrNotInCluster
			}

			oktetoURL := okteto.CloudURL
			if len(args) > 0 {
				u, err := url.Parse(args[0])
				if err != nil {
					return fmt.Errorf("malformed login URL")
				}

				if u.Scheme == "" {
					u.Scheme = "https"
				}

				oktetoURL = u.String()
			}

			log.Debugf("authenticating with %s", oktetoURL)
			var u *okteto.User
			var err error

			if len(token) > 0 {
				log.Debugf("authenticating with an api token")
				u, err = login.WithToken(ctx, oktetoURL, token)
			} else {
				log.Debugf("authenticating with the browser")
				u, err = withBrowser(ctx, oktetoURL)
			}

			if err != nil {
				analytics.TrackLogin(false, "", "", "", "")
				return err
			}

			log.Info("authenticated user %s", u.ID)
			if oktetoURL == okteto.CloudURL {
				log.Success("Logged in as %s", u.GithubID)
			} else {
				log.Success("Logged in as %s @ %s", u.GithubID, oktetoURL)
			}

			log.Hint("    Run `okteto namespace` to switch your context and download your Kubernetes credentials.")
			if u.New {
				analytics.TrackSignup(true, u.ID)
			}
			analytics.TrackLogin(true, u.Name, u.Email, u.ID, u.GithubID)
			return nil
		},
	}

	cmd.Flags().StringVarP(&token, "token", "t", "", "API token for authentication")
	return cmd
}

func withBrowser(ctx context.Context, oktetoURL string) (*okteto.User, error) {
	port, err := model.GetAvailablePort()

	if err != nil {
		log.Infof("couldn't access the network: %s", err)
		return nil, fmt.Errorf("couldn't access the network")
	}

	h, err := login.StartWithBrowser(ctx, oktetoURL, port)
	if err != nil {
		log.Infof("couldn't start the login process: %s", err)
		return nil, fmt.Errorf("couldn't start the login process, please try again")
	}

	authorizationURL := h.AuthorizationURL()
	fmt.Println("Authentication will continue in your default browser")
	if err := open.Start(authorizationURL); err != nil {
		log.Errorf("Something went wrong opening your browser: %s\n", err)
	}
	fmt.Printf("You can also open a browser and navigate to the following address:\n")
	fmt.Println(authorizationURL)

	return login.EndWithBrowser(ctx, h)
}
