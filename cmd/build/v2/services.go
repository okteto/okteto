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
	"strings"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"golang.org/x/sync/errgroup"
)

// GetServicesToBuild returns the services it has to build if they are not already built
// this function is called from outside the build cmd
func (bc *OktetoBuilder) GetServicesToBuild(ctx context.Context, manifest *model.Manifest, svcsToDeploy []string) ([]string, error) {
	buildManifest := manifest.Build

	if len(buildManifest) == 0 {
		oktetoLog.Information("Build section is not defined in your okteto manifest")
		return nil, nil
	}

	// create a spinner to be loaded before checking if images needs to be built
	oktetoLog.Spinner("Checking images to build...")

	// start the spinner
	oktetoLog.StartSpinner()

	// stop the spinner
	defer oktetoLog.StopSpinner()

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
			oktetoLog.Debug("Skipping service '%s' because it is not in the list of services to deploy", service)
			continue
		}
		svc := service

		g.Go(func() error {
			return bc.checkServiceToBuild(svc, manifest, toBuildCh)
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	close(toBuildCh)

	if len(toBuildCh) == 0 {
		oktetoLog.Information("Images were already built. To rebuild your images run 'okteto build' or 'okteto deploy --build'")
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

// checkServiceToBuild looks for the service image reference at the registry and adds it to the buildCh if is not found
func (bc *OktetoBuilder) checkServiceToBuild(service string, manifest *model.Manifest, buildCh chan string) error {
	buildInfo := manifest.Build[service].Copy()
	isStack := manifest.Type == model.StackType
	if isStack && okteto.IsOkteto() && !bc.Registry.IsOktetoRegistry(buildInfo.Image) {
		buildInfo.Image = ""
	}
	buildHash := getBuildHashFromCommit(buildInfo, bc.Config.GetGitCommit())
	imageChecker := getImageChecker(buildInfo, bc.Config, bc.Registry)
	imageWithDigest, err := imageChecker.getImageDigestReferenceForService(manifest.Name, service, buildInfo, buildHash)
	if oktetoErrors.IsNotFound(err) {
		oktetoLog.Debug("image not found, building image")
		buildCh <- service
		return nil
	} else if err != nil {
		return err
	}
	oktetoLog.Debug("Skipping build for image for service: %s", service)

	bc.SetServiceEnvVars(service, imageWithDigest)

	if manifest.Deploy != nil && manifest.Deploy.ComposeSection != nil && manifest.Deploy.ComposeSection.Stack != nil {
		stack := manifest.Deploy.ComposeSection.Stack
		if svc, ok := stack.Services[service]; ok && svc.Image == "" {
			stack.Services[service].Image = fmt.Sprintf("${OKTETO_BUILD_%s_IMAGE}", strings.ToUpper(strings.ReplaceAll(service, "-", "_")))
		}
	}
	return nil
}
