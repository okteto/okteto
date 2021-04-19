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

package namespace

import (
	"context"
	"fmt"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/login"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/errors"
	k8Client "github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/k8s/namespaces"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

//Namespace fetch credentials for a cluster namespace
func Namespace(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "namespace [name]",
		Short: "Downloads k8s credentials for a namespace",
		RunE: func(cmd *cobra.Command, args []string) error {

			namespace := ""
			if len(args) > 0 {
				namespace = args[0]
			}

			if err := login.WithEnvVarIfAvailable(ctx); err != nil {
				return err
			}

			err := RunNamespace(ctx, namespace)
			analytics.TrackNamespace(err == nil)
			return err
		},
	}
	return cmd
}

//RunNamespace starts the kubeconfig sequence
func RunNamespace(ctx context.Context, namespace string) error {
	if !okteto.IsAuthenticated() {
		if !askIfLogin() {
			return errors.ErrNotLogged
		}

		oktetoURL, err := askOktetoURL()
		if err != nil {
			return err
		}

		u, err := login.WithBrowser(ctx, oktetoURL)
		if err != nil {
			return err
		}
		log.Infof("authenticated user %s", u.ID)

		if oktetoURL == okteto.CloudURL {
			log.Success("Logged in as %s", u.ExternalID)
		} else {
			log.Success("Logged in as %s @ %s", u.ExternalID, oktetoURL)
		}
	}

	cred, err := okteto.GetCredentials(ctx)
	if err != nil {
		return err
	}
	if namespace == "" {
		namespace = cred.Namespace
	}

	kubeConfigFile := config.GetKubeConfigFile()
	clusterContext := okteto.GetClusterContext()

	if err = isNamespaceAvailable(ctx, namespace); err != nil {
		return err
	}

	if err := okteto.SetKubeConfig(cred, kubeConfigFile, namespace, okteto.GetUserID(), clusterContext, true); err != nil {
		return err
	}

	log.Success("Updated context '%s' in '%s'", clusterContext, kubeConfigFile)
	return nil
}

func askIfLogin() bool {
	result, err := utils.AskYesNo("Authentication required. Do you want to log into Okteto? [y/n]: ")
	if err != nil {
		return false
	}
	return result
}

//askOktetoURL prompts for okteto URL
func askOktetoURL() (string, error) {
	var oktetoURL string

	fmt.Print(fmt.Sprintf("What is the URL of your Okteto instance? [%s]: ", okteto.CloudURL))
	if _, err := fmt.Scanln(&oktetoURL); err != nil {
		oktetoURL = okteto.CloudURL
	}

	u, err := utils.ParseURL(oktetoURL)
	if err != nil {
		return "", fmt.Errorf("malformed login URL")
	}
	oktetoURL = u

	return oktetoURL, nil
}

func isNamespaceAvailable(ctx context.Context, namespace string) error {
	client, _, err := k8Client.GetLocal()
	if err != nil {
		return err
	}

	_, err = namespaces.Get(ctx, namespace, client)
	if err != nil {
		return fmt.Errorf("Cannot get '%s' namespace. Please make sure it's created and you have the right permissions.", namespace)
	}
	return nil
}
