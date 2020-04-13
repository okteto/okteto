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
	"strings"

	"github.com/okteto/okteto/cmd/namespace"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/login"
	k8Client "github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/cobra"
)

//Login starts the login handshake with github and okteto
func Login() *cobra.Command {
	token := ""
	cmd := &cobra.Command{
		Use:   "login [url]",
		Short: "Log into Okteto Cloud",
		Long: `Log into Okteto Cloud

Run
    $ okteto login

and this command will open your browser to ask your authentication details and retreive your API token. You can script it by using the --token parameter.

By default, this will log into cloud.okteto.com. If you want to log into your Okteto Enterprise instance, specify a URL. For example, run

    $ okteto login https://okteto.example.com

to log in to a Okteto Enterprise instance running at okteto.example.com.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			if len(token) == 0 && k8Client.InCluster() {
				return fmt.Errorf("this command is not supported without the '--token' flag from inside a pod")
			}

			oktetoURL := okteto.CloudURL
			if len(args) > 0 {
				u, err := parseURL(args[0])
				if err != nil {
					return fmt.Errorf("malformed login URL")
				}

				oktetoURL = u
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

			log.Infof("authenticated user %s", u.ID)

			if oktetoURL == okteto.CloudURL {
				log.Success("Logged in as %s", u.GithubID)
			} else {
				log.Success("Logged in as %s @ %s", u.GithubID, oktetoURL)
			}

			err = namespace.RunNamespace(ctx, "")
			if err != nil {
				log.Infof("error fetching your Kubernetes credentials: %s", err)
				log.Hint("    Run `okteto namespace` to switch your context and download your Kubernetes credentials.")
			} else {
				log.Hint("    Run 'okteto namespace' every time you need to activate your Okteto context again.")
			}
			if u.New {
				analytics.TrackSignup(true, u.ID)
			}
			analytics.TrackLogin(true, u.Name, u.Email, u.ID, u.GithubID)
			return nil
		},
	}

	cmd.Flags().StringVarP(&token, "token", "t", "", "API token for authentication.  (optional)")
	return cmd
}

func withBrowser(ctx context.Context, oktetoURL string) (*okteto.User, error) {
	h, err := login.StartWithBrowser(ctx, oktetoURL)
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

func parseURL(u string) (string, error) {
	url, err := url.Parse(u)
	if err != nil {
		return "", fmt.Errorf("%s is not a valid URL", u)
	}

	if url.Scheme == "" {
		url.Scheme = "https"
	}

	return strings.TrimRight(url.String(), "/"), nil
}
