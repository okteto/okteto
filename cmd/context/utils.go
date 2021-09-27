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

package context

import (
	"context"
	"fmt"
	"net/url"

	"github.com/okteto/okteto/pkg/analytics"
	okContext "github.com/okteto/okteto/pkg/cmd/context"
	"github.com/okteto/okteto/pkg/cmd/login"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
)

type SelectItem struct {
	Name   string
	Enable bool
}

func saveOktetoContext(ctx context.Context) error {
	// if ctxOptions.clusterType == okteto.CloudContext || ctxOptions.clusterType == okteto.EnterpriseContext {
	// 	err := okContext.SaveOktetoContext(ctx, ctxOptions.Namespace)
	// 	if err != nil {
	// 		return err
	// 	}
	// } else {
	// 	err := okContext.SaveK8sContext(ctx, cluster, ctxOptions.Namespace)
	// 	if err != nil {
	// 		return err
	// 	}
	// }
	return nil
}

func authenticateToOktetoCluster(ctx context.Context, oktetoURL, token string) error {
	var user *okteto.User
	var err error
	if len(token) > 0 {
		log.Infof("authenticating with an api token")
		user, err = login.WithToken(ctx, oktetoURL, token)
		if err != nil {
			return err
		}
	} else if okContext.HasBeenLogged(oktetoURL) {
		log.Infof("re-authenticating with saved token")
		token = okContext.GetApiToken(oktetoURL)
		user, err = login.WithToken(ctx, oktetoURL, token)
		if err != nil {
			log.Infof("saved token is wrong. Authenticating with browser code")
			user, err = login.WithBrowser(ctx, oktetoURL)
			if err != nil {
				return err
			}
		}
	} else {
		log.Infof("authenticating with browser code")
		user, err = login.WithBrowser(ctx, oktetoURL)
		if err != nil {
			return err
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

	return nil
}

func getKubernetesContextList() []string {
	contextList := make([]string, 0)
	kubeConfigFile := config.GetKubeconfigPath()
	config, err := okteto.GetKubeconfig(kubeConfigFile)
	if err != nil {
		return contextList
	}
	for name := range config.Clusters {
		if _, ok := config.Clusters[name].Extensions["okteto"]; ok {
			continue
		}
		contextList = append(contextList, name)
	}
	return contextList
}

func isOktetoCluster(option string) bool {
	return option == "Okteto Cloud" || option == "Okteto Enterprise"
}

func getOktetoClusterUrl(ctx context.Context, option string) string {
	if option == "Okteto Cloud" {
		return okteto.CloudURL
	}

	return askForLoginURL(ctx)
}

func askForLoginURL(_ context.Context) string {
	//TODO: migrate from token to context at the beginning of the main command!
	clusterURL := okteto.GetURL()
	if clusterURL == "" || clusterURL == "na" {
		clusterURL = okteto.CloudURL
	}
	fmt.Printf("What is the URL of your Okteto Cluster? [%s]: ", clusterURL)
	fmt.Scanln(&clusterURL)

	url, err := url.Parse(clusterURL)
	if err != nil {
		return ""
	}
	if url.Scheme == "" {
		url.Scheme = "https"
	}
	return url.String()
}

func isValidCluster(cluster string) bool {
	for _, c := range getKubernetesContextList() {
		if cluster == c {
			return true
		}
	}
	return false
}
