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
	"fmt"
	"net/url"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
)

type SelectItem struct {
	Name   string
	Enable bool
}

func getKubernetesContextList() []string {
	contextList := make([]string, 0)
	kubeconfigFile := config.GetKubeconfigPath()
	cfg := kubeconfig.Get(kubeconfigFile)
	if cfg == nil {
		return contextList
	}
	for name := range cfg.Contexts {
		if _, ok := cfg.Contexts[name].Extensions[model.OktetoExtension]; ok {
			continue
		}
		contextList = append(contextList, name)
	}
	return contextList
}

func isOktetoCluster(option string) bool {
	return option == "Okteto Cloud" || option == "Okteto Enterprise"
}

func getOktetoClusterUrl(option string) string {
	if option == "Okteto Cloud" {
		return okteto.CloudURL
	}

	return askForOktetoURL()
}

func askForOktetoURL() string {
	octx := okteto.Context()
	clusterURL := octx.Name
	if !okteto.IsOktetoURL(octx.Name) {
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
