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

// GetSvcsToBuildFromList returns the builds from a list and all its
func (b *ManifestBuild) GetSvcsToBuildFromList(toBuild []string) []string {
	initialSvcsToBuild := toBuild
	svcsToBuildWithDependencies := utils.GetDependentNodes(b.toGraph(), toBuild)
	if len(initialSvcsToBuild) != len(svcsToBuildWithDependencies) {
		dependantBuildImages := utils.GetListDiff(initialSvcsToBuild, svcsToBuildWithDependencies)
		oktetoLog.Warning("The following build images need to be built because of dependencies: [%s]", strings.Join(dependantBuildImages, ", "))
	}
	return svcsToBuildWithDependencies
}

func (b ManifestBuild) toGraph() utils.Graph {
	g := utils.Graph{}
	for k, v := range b {
		g[k] = v.DependsOn
	}
	return g
}
