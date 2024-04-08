// Copyright 2024 The Okteto Authors
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

package deploy

import (
	"context"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/types"
)

// buildImages it collects all the images that need to be built during the deploy phase and builds them
func buildImages(ctx context.Context, builder builderInterface, deployOptions *Options) error {
	var stackServicesWithBuild map[string]bool

	if stack := deployOptions.Manifest.GetStack(); stack != nil {
		stackServicesWithBuild = stack.GetServicesWithBuildSection()
	}

	allServicesWithBuildSection := deployOptions.Manifest.GetBuildServices()
	oktetoManifestServicesWithBuild := setDifference(allServicesWithBuildSection, stackServicesWithBuild) // Warning: this way of getting the oktetoManifestServicesWithBuild is highly dependent on the manifest struct as it is now. We are assuming that: *okteto* manifest build = manifest build - stack build section
	servicesToDeployWithBuild := setIntersection(allServicesWithBuildSection, sliceToSet(deployOptions.ServicesToDeploy))
	// We need to build:
	// - All the services that have a build section defined in the *okteto* manifest
	// - Services from *deployOptions.servicesToDeploy* that have a build section

	servicesToBuildSet := setUnion(oktetoManifestServicesWithBuild, servicesToDeployWithBuild)

	if deployOptions.Build {
		buildOptions := &types.BuildOptions{
			EnableStages: true,
			Manifest:     deployOptions.Manifest,
			CommandArgs:  setToSlice(servicesToBuildSet),
		}
		oktetoLog.Debug("force build from manifest definition")
		if errBuild := builder.Build(ctx, buildOptions); errBuild != nil {
			return errBuild
		}
	} else {
		servicesToBuild, err := builder.GetServicesToBuildDuringDeploy(ctx, deployOptions.Manifest, setToSlice(servicesToBuildSet))
		if err != nil {
			return err
		}

		if len(servicesToBuild) != 0 {
			buildOptions := &types.BuildOptions{
				EnableStages: true,
				Manifest:     deployOptions.Manifest,
				CommandArgs:  servicesToBuild,
			}

			if errBuild := builder.Build(ctx, buildOptions); errBuild != nil {
				return errBuild
			}
		}
	}

	return nil
}

func sliceToSet[T comparable](slice []T) map[T]bool {
	set := make(map[T]bool)
	for _, value := range slice {
		set[value] = true
	}
	return set
}

func setToSlice[T comparable](set map[T]bool) []T {
	slice := make([]T, 0, len(set))
	for value := range set {
		slice = append(slice, value)
	}
	return slice
}

func setIntersection[T comparable](set1, set2 map[T]bool) map[T]bool {
	intersection := make(map[T]bool)
	for value := range set1 {
		if _, ok := set2[value]; ok {
			intersection[value] = true
		}
	}
	return intersection
}

func setUnion[T comparable](set1, set2 map[T]bool) map[T]bool {
	union := make(map[T]bool)
	for value := range set1 {
		union[value] = true
	}
	for value := range set2 {
		union[value] = true
	}
	return union
}

func setDifference[T comparable](set1, set2 map[T]bool) map[T]bool {
	difference := make(map[T]bool)
	for value := range set1 {
		if _, ok := set2[value]; !ok {
			difference[value] = true
		}
	}
	return difference
}
