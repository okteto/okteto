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

	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/prompt"
)

var (
	cloudOption = fmt.Sprintf("%s (Okteto Cloud)", okteto.CloudURL)
	newOEOption = "Create new context"
)

// contextSelector represents the context information
type contextSelector struct {
	Name      string
	Label     string
	Enable    bool
	IsOkteto  bool
	Namespace string
	Builder   string
	Registry  string
}

func getContextsSelection(ctxOptions *ContextOptions) []contextSelector {
	k8sClusters := make([]string, 0)
	if !ctxOptions.OnlyOkteto {
		k8sClusters = getKubernetesContextList(true)
	}
	clusters := make([]contextSelector, 0)

	clusters = append(clusters, contextSelector{Name: okteto.CloudURL, Label: cloudOption, Enable: true, IsOkteto: true})
	clusters = append(clusters, getOktetoClusters(true)...)
	if len(k8sClusters) > 0 {
		clusters = append(clusters, getK8sClusters(k8sClusters)...)
	}
	clusters = append(clusters, []contextSelector{
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

func getOktetoClusters(skipCloud bool) []contextSelector {
	orderedOktetoClusters := make([]contextSelector, 0)
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
			contextSelector{
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

func getK8sClusters(k8sClusters []string) []contextSelector {
	orderedK8sClusters := make([]contextSelector, 0)
	for _, k8sCluster := range k8sClusters {
		orderedK8sClusters = append(orderedK8sClusters, contextSelector{
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

// getSelectorItemsFromContextSelector returns a list of selectable items and the initial position for the current selected
func getSelectorItemsFromContextSelector(items []contextSelector) ([]prompt.SelectorItem, int) {
	currentContextName := okteto.ContextStore().CurrentContext
	s := make([]prompt.SelectorItem, 0)
	currentIndx := -1
	for indx, item := range items {
		s = append(s, prompt.SelectorItem{
			Name:   item.Name,
			Label:  item.Label,
			Enable: item.Enable,
		})
		// when the name is the current context, set the index so it will be pre-selected at prompt
		if currentContextName == item.Name {
			currentIndx = indx
		}
	}
	return s, currentIndx
}

func isOktetoContextSelected(contexts []contextSelector, selected string) bool {
	for _, i := range contexts {
		if i.Name == selected {
			return i.IsOkteto
		}
	}
	return false
}
