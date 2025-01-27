// Copyright 2025 The Okteto Authors
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

package up

import (
	"context"
	"errors"

	buildv2 "github.com/okteto/okteto/cmd/build/v2"
	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/types"
)

type upBuilder struct {
	builder  builderInterface
	registry registryInterface
	manifest *model.Manifest
	devName  string
}

func newUpBuilder(m *model.Manifest, devName string, builder builderInterface, reg registryInterface) *upBuilder {
	return &upBuilder{
		builder:  builder,
		manifest: m,
		devName:  devName,
		registry: reg,
	}
}

func (ub *upBuilder) build(ctx context.Context) error {
	// check if the dev image uses the same image as the original service
	if ub.manifest.Dev[ub.devName].Image == "" {
		return nil
	}

	// check if the dev image has a OKTETO_BUILD syntax
	buildSvc, err := ub.builder.GetSvcToBuildFromRegex(ub.manifest, ub.getBuildSvcFromDev)
	if err != nil && !errors.Is(err, buildv2.ErrImageIsNotAOktetoBuildSyntax) {
		return err
	}

	// check if the dev image is present on the build section
	if buildSvc == "" {
		devImage := ub.manifest.Dev[ub.devName].Image
		buildSvc = ub.getBuildServiceFromImage(devImage)
		// it's not present on the build section, we don't need to build anything
		if buildSvc == "" {
			return nil
		}
	}

	// get all the services that need to be built
	visited := make(map[string]bool)
	dependentSvcs := ub.getDependentServices(buildSvc, ub.manifest.Build, visited)

	toBuildCheck := []string{buildSvc}
	toBuildCheck = append(toBuildCheck, dependentSvcs...)

	// check if the services are already built
	svcsToBuild, err := ub.builder.GetServicesToBuildDuringExecution(ctx, ub.manifest, toBuildCheck)
	if err != nil {
		return err
	}
	if len(svcsToBuild) == 0 {
		return nil
	}
	buildOptions := &types.BuildOptions{
		CommandArgs: svcsToBuild,
		Manifest:    ub.manifest,
	}
	return ub.builder.Build(ctx, buildOptions)
}

func (ub *upBuilder) getBuildSvcFromDev(manifest *model.Manifest) string {
	dev := ub.manifest.Dev[ub.devName]
	if dev == nil {
		return ""
	}
	return dev.Image
}

func (ub *upBuilder) getBuildServiceFromImage(image string) string {
	for svc, info := range ub.manifest.Build {
		if ub.registry.ExpandImage(info.Image) == ub.registry.ExpandImage(image) {
			return svc
		}
	}
	return ""
}

func (ub *upBuilder) getDependentServices(buildSvc string, bm build.ManifestBuild, visited map[string]bool) []string {
	// If the key has been visited, we return an empty list
	if visited[buildSvc] {
		return nil
	}
	visited[buildSvc] = true

	var result []string
	for _, dep := range bm[buildSvc].DependsOn {
		result = append(result, dep)

		subDeps := ub.getDependentServices(dep, bm, visited)
		result = append(result, subDeps...)
	}

	return result
}
