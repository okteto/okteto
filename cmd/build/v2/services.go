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

	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/format"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"golang.org/x/sync/errgroup"
)

// GetServicesToBuild returns the services it has to build if they are not already built
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
	toBuild := make(chan string, len(svcToDeployMap))
	g, _ := errgroup.WithContext(ctx)
	for service := range buildManifest {
		if _, ok := svcToDeployMap[service]; !ok {
			oktetoLog.Debug("Skipping service '%s' because it is not in the list of services to deploy", service)
			continue
		}
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
		if _, ok := svcToDeployMap[svc]; len(svcsToDeploy) > 0 && !ok {
			continue
		}
		svcsToBuildList = append(svcsToBuildList, svc)
	}
	return svcsToBuildList, nil
}

func (bc *OktetoBuilder) checkServicesToBuild(service string, manifest *model.Manifest, ch chan string) error {
	buildInfo := manifest.Build[service].Copy()
	isStack := manifest.Type == model.StackType
	if isStack && okteto.IsOkteto() && !bc.Registry.IsOktetoRegistry(buildInfo.Image) {
		buildInfo.Image = ""
	}

	tagsToCheck := bc.tagsToCheck(manifest.Name, service, buildInfo)
	if len(tagsToCheck) == 0 {
		return fmt.Errorf("error getting the image name for the service '%s'. Please specify the full name of the image when using a cluster that doesn't have Okteto installed", service)
	}

	imageWithDigest, err := bc.isImageBuilt(tagsToCheck)
	if oktetoErrors.IsNotFound(err) {
		oktetoLog.Debug("image not found, building image")
		ch <- service
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

// isImageBuilt checks a list of tags that the same image can refer to
// if one of them is found it returns the image with the digest
func (bc *OktetoBuilder) isImageBuilt(tags []string) (string, error) {
	for _, tag := range tags {
		imageWithDigest, err := bc.Registry.GetImageTagWithDigest(tag)

		if err != nil {
			if oktetoErrors.IsNotFound(err) {
				continue
			}
			// return error if the registry doesn't send a not found error
			return "", fmt.Errorf("error checking image at registry %s: %v", tag, err)
		}
		return imageWithDigest, nil
	}
	return "", fmt.Errorf("not found")
}

// getTagToBuild gets all the possible tags that could be built and builts the one with more priority
// okteto.global + hash / okteto.dev + hash / okteto.global + tag / okteto.dev + tag
func (bc *OktetoBuilder) getTagToBuild(manifestName, svcName string, b *model.BuildInfo) string {
	return bc.tagsToCheck(manifestName, svcName, b)[0]
}

func (bc *OktetoBuilder) tagsToCheck(manifestName, svcName string, b *model.BuildInfo) []string {
	targetRegistries := []string{constants.DevRegistry}
	sha := ""
	if bc.Config.HasGlobalAccess() && bc.Config.IsCleanProject() {
		targetRegistries = []string{constants.GlobalRegistry, constants.DevRegistry}
		sha = bc.Config.GetHash()
	}

	// manifestName can be not sanitized when option name is used at deploy
	sanitizedName := format.ResourceK8sMetaString(manifestName)
	tagsToCheck := []string{}

	switch {
	case !okteto.IsOkteto():
		tagsToCheck = append(tagsToCheck, b.Image)
		return tagsToCheck
	case (shouldBuildFromDockerfile(b) && shouldAddVolumeMounts(b)) || shouldAddVolumeMounts(b):
		if sha != "" {
			for _, targetRegistry := range targetRegistries {
				tagsToCheck = append(tagsToCheck, getImageFromTmpl(targetRegistry, sanitizedName, svcName, sha))
			}
		}
		for _, targetRegistry := range targetRegistries {
			tagsToCheck = append(tagsToCheck, getImageFromTmpl(targetRegistry, sanitizedName, svcName, model.OktetoImageTagWithVolumes))
		}
		return tagsToCheck
	case b.Image != "" && shouldBuildFromDockerfile(b):
		tagsToCheck = append(tagsToCheck, b.Image)
		return tagsToCheck
	case shouldBuildFromDockerfile(b):
		if sha != "" {
			for _, targetRegistry := range targetRegistries {
				tagsToCheck = append(tagsToCheck, getImageFromTmpl(targetRegistry, sanitizedName, svcName, sha))
			}
		}
		for _, targetRegistry := range targetRegistries {
			tagsToCheck = append(tagsToCheck, getImageFromTmpl(targetRegistry, sanitizedName, svcName, model.OktetoDefaultImageTag))
		}
		return tagsToCheck
	case b.Image != "":
		tagsToCheck = append(tagsToCheck, b.Image)
		return tagsToCheck
	default:
		oktetoLog.Infof("could not build service %s, due to not having Dockerfile defined or volumes to include", svcName)
	}
	return tagsToCheck
}

func getImageFromTmpl(targetRegistry, repoName, svcName, tag string) string {
	return fmt.Sprintf("%s/%s-%s:%s", targetRegistry, repoName, svcName, tag)
}

func (bc *OktetoBuilder) checkIfCommitIsAlreadyBuilt(manifestName, svcName, sha string, noCache bool) (string, bool) {
	if !bc.Config.IsCleanProject() {
		return "", false
	}
	if noCache {
		return "", false
	}

	targetRegistries := []string{constants.GlobalRegistry, constants.DevRegistry}
	tagsToCheck := []string{}
	// manifestName can be not sanitized when option name is used at deploy
	sanitizedName := format.ResourceK8sMetaString(manifestName)
	for _, targetRegistry := range targetRegistries {
		tagsToCheck = append(tagsToCheck, getImageFromTmpl(targetRegistry, sanitizedName, svcName, sha))
	}
	imageTag, err := bc.isImageBuilt(tagsToCheck)
	if err != nil {
		return "", false
	}
	return imageTag, true
}
