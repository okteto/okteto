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

package build

import (
	"fmt"
	"strings"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model/utils"
)

// ManifestBuild defines all the build section
type ManifestBuild map[string]*Info

// Validate validates the build section of the manifest
func (b *ManifestBuild) Validate() error {
	for k, v := range *b {
		if v == nil {
			return fmt.Errorf("manifest validation failed: service '%s' build section not defined correctly", k)
		}
	}

	cycle := utils.GetDependentCyclic(b.toGraph())
	if len(cycle) == 1 { // depends on the same node
		return fmt.Errorf("manifest build validation failed: image '%s' is referenced on its dependencies", cycle[0])
	} else if len(cycle) > 1 {
		svcsDependents := fmt.Sprintf("%s and %s", strings.Join(cycle[:len(cycle)-1], ", "), cycle[len(cycle)-1])
		return fmt.Errorf("manifest validation failed: cyclic dependendecy found between %s", svcsDependents)
	}
	return nil
}

// GetSvcsToBuildFromList returns the builds from a list and all its dependencies
func (b *ManifestBuild) GetSvcsToBuildFromList(toBuild []string) []string {
	initialSvcsToBuild := toBuild
	svcsToBuildWithDependencies := getDependentNodes(b.toGraph(), toBuild)
	if len(initialSvcsToBuild) != len(svcsToBuildWithDependencies) {
		dependantBuildImages := getListDiff(initialSvcsToBuild, svcsToBuildWithDependencies)
		oktetoLog.Warning("The following build images need to be built because of dependencies: [%s]", strings.Join(dependantBuildImages, ", "))
	}
	return svcsToBuildWithDependencies
}

func getListDiff(l1, l2 []string) []string {
	var (
		longerList  []string
		shorterList []string
	)
	if len(l1) < len(l2) {
		shorterList = l1
		longerList = l2

	} else {
		shorterList = l2
		longerList = l1
	}

	shorterListSet := map[string]bool{}
	for _, svc := range shorterList {
		shorterListSet[svc] = true
	}
	added := []string{}
	for _, svcName := range longerList {
		if _, ok := shorterListSet[svcName]; ok {
			added = append(added, svcName)
		}
	}
	return added
}

func getDependentNodes(g utils.Graph, startingNodes []string) []string {
	initialLength := len(startingNodes)
	svcsToDeploySet := map[string]bool{}
	for _, svc := range startingNodes {
		svcsToDeploySet[svc] = true
	}
	for _, svcToDeploy := range startingNodes {
		for _, dependentSvc := range g[svcToDeploy] {
			if _, ok := svcsToDeploySet[dependentSvc]; ok {
				continue
			}
			startingNodes = append(startingNodes, dependentSvc)
			svcsToDeploySet[dependentSvc] = true
		}
	}
	if initialLength != len(startingNodes) {
		return getDependentNodes(g, startingNodes)
	}
	return startingNodes
}

func (b ManifestBuild) toGraph() utils.Graph {
	g := utils.Graph{}
	for k, v := range b {
		g[k] = v.DependsOn
	}
	return g
}
