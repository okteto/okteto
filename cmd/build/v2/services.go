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
	"github.com/okteto/okteto/pkg/format"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"golang.org/x/sync/errgroup"
)

// GetServicesToBuild returns the services it has to build if they are not already built
func (bc *OktetoBuilder) GetServicesToBuild(ctx context.Context, manifest *model.Manifest, svcToDeploy []string) ([]string, error) {
	buildManifest := manifest.Build

	if len(buildManifest) == 0 {
		oktetoLog.Information("Build section is not defined in your okteto manifest")
		return nil, nil
	}

	// check if images are at registry (global or dev) and set envs or send to build
	toBuild := make(chan string, len(buildManifest))
	g, _ := errgroup.WithContext(ctx)
	for service := range buildManifest {

		svc := service
		g.Go(func() error {
			digest, err := bc.getDigestFromService(svc, manifest)
			if err != nil {
				return err
			}
			if digest == "" {
				toBuild <- svc
				return nil
			}
			oktetoLog.Debug("Skipping build for image for service")

			bc.SetServiceEnvVars(svc, digest)

			if manifest.Deploy != nil && manifest.Deploy.ComposeSection != nil && manifest.Deploy.ComposeSection.Stack != nil {
				stack := manifest.Deploy.ComposeSection.Stack
				if svcInfo, ok := stack.Services[svc]; ok && svcInfo.Image == "" {
					stack.Services[svc].Image = fmt.Sprintf("${OKTETO_BUILD_%s_IMAGE}", strings.ToUpper(strings.ReplaceAll(svc, "-", "_")))
				}
			}
			return nil
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

func (bc *OktetoBuilder) getDigestFromService(service string, manifest *model.Manifest) (string, error) {
	buildInfo := manifest.Build[service].Copy()

	tags := getToBuildTags(manifest.Name, service, buildInfo)
	if len(tags) == 0 {
		return "", fmt.Errorf("error getting the image name for the service '%s'. Please specify the full name of the image when using a cluster that doesn't have Okteto installed", service)
	}

	for _, tag := range tags {
		imageWithDigest, err := bc.Registry.GetImageTagWithDigest(tag)
		if oktetoErrors.IsNotFound(err) {
			oktetoLog.Debugf("image not found for tag %s, building image", tag)
			continue
		} else if err != nil {
			return "", fmt.Errorf("error checking image at registry %s: %w", tag, err)
		}
		return imageWithDigest, nil
	}

	return "", nil
}

func getToBuildTags(manifestName, svcName string, b *model.BuildInfo) []string {
	if !okteto.IsOkteto() {
		if b.Image != "" {
			return []string{b.Image}
		}
		return []string{}
	}

	if registry.IsOktetoRegistry(b.Image) {
		return []string{b.Image}
	}

	possibleTags := []string{}
	// manifestName can be not sanitized when option name is used at deploy
	sanitizedName := format.ResourceK8sMetaString(manifestName)

	targetRegistries := []string{okteto.DevRegistry, okteto.GlobalRegistry}
	for _, targetRegistry := range targetRegistries {
		if shouldAddVolumeMounts(b) {
			possibleTags = append(possibleTags, fmt.Sprintf("%s/%s-%s:%s", targetRegistry, sanitizedName, svcName, model.OktetoImageTagWithVolumes))
			continue
		}
		if shouldBuildFromDockerfile(b) {
			possibleTags = append(possibleTags, fmt.Sprintf("%s/%s-%s:%s", targetRegistry, sanitizedName, svcName, model.OktetoDefaultImageTag))
		}
	}
	if b.Image != "" && !shouldAddVolumeMounts(b) {
		possibleTags = append(possibleTags, b.Image)
	}
	return possibleTags
}
