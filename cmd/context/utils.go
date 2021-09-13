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

	"github.com/manifoldco/promptui"
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
		ctxOptions.isOktetoCluster = true
		cluster = getOktetoClusterUrl(ctx, cluster)
		err := authenticateToOktetoCluster(ctx, cluster, ctxOptions.Token)
		if err != nil {
			return "", err
		}
		okteto.SetIsOktetoCluster(ctxOptions.isOktetoCluster)
	} else {
		ctxOptions.isOktetoCluster = false
		err := okContext.CopyK8sClusterConfigToOktetoContext(cluster)
		if err != nil {
			return "", err
		}
		okteto.SetIsOktetoCluster(ctxOptions.isOktetoCluster)
	}
	return cluster, nil
}

func saveOktetoContext(ctx context.Context, cluster string) error {
	if isOptionAnOktetoCluster(cluster) {
		err := okContext.SaveOktetoContext(ctx)
		if err != nil {
			return err
		}
	} else {
		err := okContext.SaveK8sContext(ctx, cluster)
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
	clusterList := getClusterListOptions()

	prompt := promptui.Select{
		Label: "Select the cluster you want to point to:",
		Items: clusterList,
		Size:  len(clusterList),
		Templates: &promptui.SelectTemplates{
			Label:    "{{ .Name }}",
			Selected: "{{if .Enable}} ✓  {{ .Name | oktetoblue }}{{else}}{{ .Name | oktetoblue}}{{end}}",
			Active:   fmt.Sprintf("{{if .Enable}}%s {{ .Name | oktetoblue }}{{else}}{{ .Name | oktetoblue}}{{end}}", promptui.IconSelect),
			Inactive: "{{if .Enable}}  {{ .Name | oktetoblue}}{{else}}{{ .Name | oktetoblue}}{{end}}",
			FuncMap:  promptui.FuncMap,
		},
	}

	prompt.Templates.FuncMap["oktetoblue"] = log.BlueString

	i, _, err := prompt.RunCursorAt(1, 0)
	if err != nil {
		log.Infof("invalid cluster: %s", err)
		return "", fmt.Errorf("invalid cluster")
	}
	if clusterList[i].Name == "Okteto clusters:" || clusterList[i].Name == "Kubernetes clusters:" {
		log.Infof("invalid cluster")
		return "", fmt.Errorf("invalid cluster")
	}
	return clusterList[i].Name, nil
}

func getClusterListOptions() []SelectItem {
	k8sClusters := getClusterList()
	clusterOptions := []SelectItem{
		{
			Name:   "Okteto clusters:",
			Enable: false,
		},
		{
			Name:   "Okteto cloud",
			Enable: true,
		},
		{
			Name:   "Okteto enterprise",
			Enable: true,
		},
	}
	if len(k8sClusters) > 0 {
		clusterOptions = append(clusterOptions, SelectItem{
			Name:   "Kubernetes clusters:",
			Enable: false,
		})
	}
	for _, k8sCluster := range k8sClusters {
		clusterOptions = append(clusterOptions, SelectItem{
			Name:   k8sCluster,
			Enable: true,
		})
	}
	return clusterOptions
}

func isOptionAnOktetoCluster(option string) bool {
	return option == "Okteto cloud" || option == "Okteto enterprise" || isURL(option)
}

func getOktetoClusterUrl(ctx context.Context, option string) string {
	if option == "Okteto cloud" {
		return okteto.CloudURL
	}

	return AskForLoginURL(ctx)
}

func isURL(okContext string) bool {
	u, err := url.Parse(okContext)
	return err == nil && u.Scheme != "" && u.Host != ""
}

func AskForLoginURL(ctx context.Context) string {
	url := okteto.GetURL()
	if url == "" || url == "na" {
		url = okteto.CloudURL
	}
	fmt.Printf("What is the URL of your Okteto instance? [%s]: ", url)
	fmt.Scanln(&url)
	return url
}

func isValidCluster(cluster string) bool {
	for _, c := range getClusterList() {
		if cluster == c {
			return true
		}
	}
	return false
}
