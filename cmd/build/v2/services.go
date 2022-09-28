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

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"golang.org/x/sync/errgroup"
)

// GetServicesToBuild returns the services it has to built because they are not already built
func (bc *OktetoBuilder) GetServicesToBuild(ctx context.Context, manifest *model.Manifest, svcToDeploy []string) ([]string, error) {
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

	svcToDeployMap := map[string]bool{}
	for _, svc := range svcToDeploy {
		svcToDeployMap[svc] = true
	}
	svcsToBuildList := []string{}
	for svc := range toBuild {
		if _, ok := svcToDeployMap[svc]; len(svcToDeploy) > 0 && !ok {
			continue
		}
		svcsToBuildList = append(svcsToBuildList, svc)
	}
	return svcsToBuildList, nil
}

func (bc *OktetoBuilder) checkServicesToBuild(service string, manifest *model.Manifest, ch chan string) error {
	buildInfo := manifest.Build[service].Copy()
	isStack := manifest.Type == model.StackType
	if isStack && okteto.IsOkteto() && !registry.IsOktetoRegistry(buildInfo.Image) {
		buildInfo.Image = ""
	}
	tag := getToBuildTag(manifest.Name, service, buildInfo)
	if tag == "" {
		return fmt.Errorf("error getting the image name for the service '%s'. Please specify the full name of the image when using a cluster that doesn't have Okteto installed", service)
	}

	imageWithDigest, err := bc.Registry.GetImageTagWithDigest(tag)
	if oktetoErrors.IsNotFound(err) {
		oktetoLog.Debug("image not found, building image")
		ch <- service
		return nil
	} else if err != nil {
		return fmt.Errorf("error checking image at registry %s: %v", tag, err)
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

func getToBuildTag(manifestName, svcName string, b *model.BuildInfo) string {
	targetRegistry := okteto.DevRegistry
	switch {
	case !okteto.IsOkteto():
		return b.Image
	case (shouldBuildFromDockerfile(b) && shouldAddVolumeMounts(b)) || shouldAddVolumeMounts(b):
		return fmt.Sprintf("%s/%s-%s:%s", targetRegistry, manifestName, svcName, model.OktetoImageTagWithVolumes)
	case b.Image != "" && shouldBuildFromDockerfile(b):
		return b.Image
	case shouldBuildFromDockerfile(b):
		return fmt.Sprintf("%s/%s-%s:%s", targetRegistry, manifestName, svcName, model.OktetoDefaultImageTag)
	case b.Image != "":
		return b.Image
	default:
		oktetoLog.Infof("could not build service %s, due to not having Dockerfile defined or volumes to include", svcName)
	}
	return ""
}
