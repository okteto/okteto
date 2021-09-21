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

	"github.com/okteto/okteto/cmd/utils"
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

func getCluster(ctx context.Context, ctxOptions *ContextOptions) (string, error) {
	cluster, err := selectCluster()
	if err != nil {
		return "", err
	}

	if isOptionAnOktetoCluster(cluster) {

		cluster = getOktetoClusterUrl(ctx, cluster)

		if cluster == okteto.CloudURL || cluster == okteto.StagingURL {
			ctxOptions.clusterType = okteto.CloudCluster
		} else {
			ctxOptions.clusterType = okteto.EnterpriseCluster
		}
		err := authenticateToOktetoCluster(ctx, cluster, ctxOptions.Token)
		if err != nil {
			return "", err
		}
	} else {
		ctxOptions.clusterType = okteto.VanillaCluster

		err := okContext.CopyK8sClusterConfigToOktetoContext(cluster)
		if err != nil {
			return "", err
		}
	}
	return cluster, nil
}

func saveOktetoContext(ctx context.Context, cluster string, ctxOptions *ContextOptions) error {
	if ctxOptions.clusterType == okteto.CloudCluster || ctxOptions.clusterType == okteto.EnterpriseCluster {
		err := okContext.SaveOktetoContext(ctx, ctxOptions.Namespace)
		if err != nil {
			return err
		}
	} else {
		err := okContext.SaveK8sContext(ctx, cluster, ctxOptions.Namespace)
		if err != nil {
			return err
		}
	}
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

	analytics.TrackLogin(true, user.Name, user.Email, user.ID, user.ExternalID)
	return nil
}

func getClusterList() []string {
	contextList := make([]string, 0)
	kubeConfigFile := config.GetKubeConfigFile()
	config, err := okteto.GetKubeConfig(kubeConfigFile)
	if err != nil {
		return contextList
	}
	for name := range config.Clusters {
		contextList = append(contextList, name)
	}
	return contextList
}

func selectCluster() (string, error) {
	clusters := []string{"Okteto Cloud", "Okteto Enterprise"}
	k8sClusters := getClusterList()
	clusters = append(clusters, k8sClusters...)
	return utils.AskForOptions(clusters, "Select the cluster you want to point to:")
}

func isOptionAnOktetoCluster(option string) bool {
	return option == "Okteto Cloud" || option == "Okteto Enterprise"
}

func getOktetoClusterUrl(ctx context.Context, option string) string {
	if option == "Okteto Cloud" {
		return okteto.CloudURL
	}

	return AskForLoginURL(ctx)
}

func isURL(okContext string) bool {
	u, err := url.Parse(okContext)
	return err == nil && u.Scheme != "" && u.Host != ""
}

func AskForLoginURL(_ context.Context) string {
	clusterURL := okteto.GetURL()
	if clusterURL == "" || clusterURL == "na" {
		clusterURL = okteto.CloudURL
	}
	fmt.Printf("What is the URL of your Okteto instance? [%s]: ", clusterURL)
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
	for _, c := range getClusterList() {
		if cluster == c {
			return true
		}
	}
	return false
}
