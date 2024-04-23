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

package v2

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/model"
	"golang.org/x/sync/errgroup"
)

// GetServicesToBuildDuringDeploy returns the services it has to build if they are not already built
// this function is called from outside the build cmd and during a "deploy operation" (up, deploy, destroy, compose).
func (bc *OktetoBuilder) GetServicesToBuildDuringDeploy(ctx context.Context, manifest *model.Manifest, svcsToDeploy []string) ([]string, error) {
	buildManifest := manifest.Build

	if len(buildManifest) == 0 {
		return nil, nil
	}

	// create a spinner to be loaded before checking if images needs to be built
	sp := bc.ioCtrl.Out().Spinner("Checking images to build...")
	sp.Start()
	defer sp.Stop()

	svcToDeployMap := map[string]bool{}
	if len(svcsToDeploy) == 0 {
		for svc := range buildManifest {
			svcToDeployMap[svc] = true
		}
	} else {
		for _, svcToDeploy := range svcsToDeploy {
			svcToDeployMap[svcToDeploy] = true
		}
	}
	// check if images are at registry (global or dev) and set envs or send to build
	toBuildCh := make(chan string, len(svcToDeployMap))
	g, _ := errgroup.WithContext(ctx)
	for service := range buildManifest {
		if _, ok := svcToDeployMap[service]; !ok {
			bc.ioCtrl.Logger().Debugf("Skipping service '%s' because it is not in the list of services to deploy", service)
			continue
		}
		svc := service

		g.Go(func() error {
			return bc.checkServiceToBuildDuringDeploy(svc, manifest, toBuildCh)
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	close(toBuildCh)

	if len(toBuildCh) == 0 {
		bc.ioCtrl.Out().Infof("Images were already built. To rebuild your images run 'okteto build' or 'okteto deploy --build'")
		if err := manifest.ExpandEnvVars(); err != nil {
			return nil, err
		}
		return nil, nil
	}

	svcsToBuildList := []string{}
	for svc := range toBuildCh {
		if _, ok := svcToDeployMap[svc]; len(svcsToDeploy) > 0 && !ok {
			continue
		}
		svcsToBuildList = append(svcsToBuildList, svc)
	}
	return svcsToBuildList, nil
}

// checkServiceToBuildDuringDeploy looks for the service image reference at the registry and adds it to the buildCh
// if is not found. This function is called during deploy operations (up, deploy, destroy and compose) to check if
// images have to be built or not. In that case, we only check the existence of "okteto" tag in the dev registry
func (bc *OktetoBuilder) checkServiceToBuildDuringDeploy(service string, manifest *model.Manifest, buildCh chan string) error {
	buildInfo := manifest.Build[service].Copy()
	isStack := manifest.Type == model.StackType
	if isStack && bc.oktetoContext.IsOkteto() && !bc.Registry.IsOktetoRegistry(buildInfo.Image) {
		buildInfo.Image = ""
	}

	imageChecker := getImageChecker(bc.Config, bc.Registry, bc.smartBuildCtrl, bc.ioCtrl.Logger())
	imageWithDigest, err := imageChecker.getImageDigestReferenceForServiceDeploy(manifest.Name, service, buildInfo)
	if oktetoErrors.IsNotFound(err) {
		bc.ioCtrl.Logger().Debug("image not found, building image")
		buildCh <- service
		return nil
	} else if err != nil {
		bc.ioCtrl.Out().Warning("could not verify if image for service %q is already in the registry. Building image...", service)
		// If there is an error trying to get the image from the registry, we just rebuild that image
		bc.ioCtrl.Logger().Debugf("unexpected error checking if the images exist: %s", err)
		buildCh <- service
		return nil
	}
	bc.ioCtrl.Logger().Debugf("Skipping build for image for service: %s", service)

	bc.SetServiceEnvVars(service, imageWithDigest)

	if manifest.Deploy != nil && manifest.Deploy.ComposeSection != nil && manifest.Deploy.ComposeSection.Stack != nil {
		stack := manifest.Deploy.ComposeSection.Stack
		if svc, ok := stack.Services[service]; ok && svc.Image == "" {
			stack.Services[service].Image = fmt.Sprintf("${OKTETO_BUILD_%s_IMAGE}", strings.ToUpper(strings.ReplaceAll(service, "-", "_")))
		}
	}
	return nil
}

func (bc *OktetoBuilder) GetServicesToBuildForImage(ctx context.Context, manifest *model.Manifest, imgFinder model.ImageFromManifest) ([]string, error) {
	img := imgFinder(manifest)
	reg := regexp.MustCompile(`OKTETO_BUILD_(\w+)_`)
	matches := reg.FindStringSubmatch(img)
	foundMatches := 2
	if len(matches) == 0 {
		return nil, nil
	}

	sanitisedToUnsanitised := map[string]string{}
	for buildSvc := range manifest.Build {
		sanitizedSvc := strings.ToUpper(strings.ReplaceAll(buildSvc, "-", "_"))
		sanitisedToUnsanitised[sanitizedSvc] = buildSvc
	}
	if len(matches) != foundMatches {
		return nil, nil
	}
	sanitisedName := matches[1]
	svc, ok := sanitisedToUnsanitised[sanitisedName]
	if !ok {
		return nil, nil
	}

	return bc.GetServicesToBuildDuringDeploy(ctx, manifest, []string{svc})
}
