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

package v2

import (
	"context"
	"fmt"
	"strings"

	"github.com/okteto/okteto/pkg/cmd/build"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/okteto/okteto/pkg/types"
	"golang.org/x/sync/errgroup"
)

// GetServicesToBuild returns the services it has to built because they are not already built
func (bc *OktetoBuilder) GetServicesToBuild(ctx context.Context, manifest *model.Manifest) ([]string, error) {
	buildManifest := manifest.Build

	// check if images are at registry (global or dev) and set envs or send to build
	toBuild := make(chan string, len(buildManifest))
	g, _ := errgroup.WithContext(ctx)
	for service := range buildManifest {
		svc := service
		g.Go(func() error {
			return bc.checkServicesToBuild(svc, manifest, toBuild)
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	close(toBuild)

	if len(toBuild) == 0 {
		oktetoLog.Information("Images were already built. To rebuild your images run 'okteto build' or 'okteto deploy --build'")
		if err := manifest.ExpandEnvVars(); err != nil {
			return nil, err
		}
		return nil, nil
	}

	svcsToBuildList := []string{}
	for svc := range toBuild {
		svcsToBuildList = append(svcsToBuildList, svc)
	}
	return svcsToBuildList, nil
}

func (bc *OktetoBuilder) checkServicesToBuild(service string, manifest *model.Manifest, ch chan string) error {
	buildInfo := manifest.Build[service]
	isStack := manifest.Type == model.StackType
	if isStack && okteto.IsOkteto() && !registry.IsOktetoRegistry(buildInfo.Image) {
		buildInfo.Image = ""
	}
	opts := build.OptsFromBuildInfo(manifest.Name, service, buildInfo, &types.BuildOptions{})

	if build.ShouldOptimizeBuild(opts) {
		oktetoLog.Debug("tag detected, optimizing sha")
		if skipBuild, err := bc.checkImageAtGlobalAndSetEnvs(service, opts); err != nil {
			return err
		} else if skipBuild {
			oktetoLog.Debugf("Skipping '%s' build. Image already exists at Okteto Registry", service)
			return nil
		}
	}

	imageWithDigest, err := bc.Registry.GetImageTagWithDigest(opts.Tag)
	if err == oktetoErrors.ErrNotFound {
		oktetoLog.Debug("image not found, building image")
		ch <- service
		return nil
	} else if err != nil {
		return fmt.Errorf("error checking image at registry %s: %v", opts.Tag, err)
	}
	oktetoLog.Debug("Skipping build for image for service")

	bc.SetServiceEnvVars(service, imageWithDigest)

	if manifest.Deploy != nil && manifest.Deploy.ComposeSection != nil && manifest.Deploy.ComposeSection.Stack != nil {
		stack := manifest.Deploy.ComposeSection.Stack
		if svc, ok := stack.Services[service]; ok && svc.Image == "" {
			stack.Services[service].Image = fmt.Sprintf("${OKTETO_BUILD_%s_IMAGE}", strings.ToUpper(strings.ReplaceAll(service, "-", "_")))
		}
	}
	return nil
}

func (bc *OktetoBuilder) checkImageAtGlobalAndSetEnvs(service string, options *types.BuildOptions) (bool, error) {
	globalReference := strings.Replace(options.Tag, okteto.DevRegistry, okteto.GlobalRegistry, 1)

	imageWithDigest, err := bc.Registry.GetImageTagWithDigest(globalReference)
	if err == oktetoErrors.ErrNotFound {
		oktetoLog.Debug("image not built at global registry, not running optimization for deployment")
		return false, nil
	}
	if err != nil {
		return false, err
	}

	bc.SetServiceEnvVars(service, imageWithDigest)
	oktetoLog.Debug("image already built at global registry, running optimization for deployment")
	return true, nil

}

// GetServicesToBuildFromSubset returns the services it has to built because they are not already built from a subset of services
func (bc *OktetoBuilder) GetServicesToBuildFromSubset(ctx context.Context, manifest *model.Manifest, subset []string) ([]string, error) {
	for name := range manifest.Build {
		needsBuild := false
		for _, toBuildName := range subset {
			if name == toBuildName {
				needsBuild = true
				break
			}
		}
		if !needsBuild {
			delete(manifest.Build, name)
		}
	}
	svcsToBuild, err := bc.GetServicesToBuild(ctx, manifest)
	if err != nil {
		return []string{}, err
	}
	return svcsToBuild, err
}
