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

package context

import (
	"fmt"
	"sort"
	"strings"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/okteto"
)

var (
	cloudOption = fmt.Sprintf("%s (Okteto Cloud)", okteto.CloudURL)
	newOEOption = "Create new context"
)

func getContextsSelection(ctxOptions *ContextOptions) []utils.SelectorItem {
	k8sClusters := make([]string, 0)
	if !ctxOptions.OnlyOkteto {
		k8sClusters = getKubernetesContextList(true)
	}
	clusters := make([]utils.SelectorItem, 0)

	clusters = append(clusters, utils.SelectorItem{Name: okteto.CloudURL, Label: cloudOption, Enable: true, IsOkteto: true})
	clusters = append(clusters, getOktetoClusters(true)...)
	if len(k8sClusters) > 0 {
		clusters = append(clusters, getK8sClusters(k8sClusters)...)
	}
	clusters = append(clusters, []utils.SelectorItem{
		{
			Label:  "",
			Enable: false,
		},
		{
			Name:   newOEOption,
			Label:  newOEOption,
			Enable: true,
		},
	}...)

	return clusters
}

func getOktetoClusters(skipCloud bool) []utils.SelectorItem {
	orderedOktetoClusters := make([]utils.SelectorItem, 0)
	ctxStore := okteto.ContextStore()
	for ctxName, okCtx := range ctxStore.Contexts {
		if !okCtx.IsOkteto {
			continue
		}
		if skipCloud && ctxName == okteto.CloudURL {
			continue
		}
		orderedOktetoClusters = append(
			orderedOktetoClusters,
			utils.SelectorItem{
				Name:      ctxName,
				Label:     ctxName,
				Enable:    true,
				IsOkteto:  true,
				Namespace: okCtx.Namespace,
				Builder:   okCtx.Builder,
				Registry:  okCtx.Registry,
			})
	}
	sort.Slice(orderedOktetoClusters, func(i, j int) bool {
		if orderedOktetoClusters[i].Name == okteto.CloudURL {
			return true
		}
		if orderedOktetoClusters[j].Name == okteto.CloudURL {
			return false
		}
		return strings.Compare(orderedOktetoClusters[i].Name, orderedOktetoClusters[j].Name) < 0
	})
	return orderedOktetoClusters
}

func getK8sClusters(k8sClusters []string) []utils.SelectorItem {
	orderedK8sClusters := make([]utils.SelectorItem, 0)
	for _, k8sCluster := range k8sClusters {
		orderedK8sClusters = append(orderedK8sClusters, utils.SelectorItem{
			Name:      k8sCluster,
			Label:     k8sCluster,
			Enable:    true,
			IsOkteto:  false,
			Namespace: getKubernetesContextNamespace(k8sCluster),
			Builder:   "docker",
			Registry:  "-",
		})
	}
	sort.Slice(orderedK8sClusters, func(i, j int) bool {
		return strings.Compare(orderedK8sClusters[i].Name, orderedK8sClusters[j].Name) < 0
	})
	return orderedK8sClusters
}
