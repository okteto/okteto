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
	"fmt"
	"strings"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
)

type imageCheckerInterface interface {
	checkIfBuildHashIsBuilt(manifestName, svcToBuild string, buildInfo *model.BuildInfo, commit string) (string, bool)
	getImageDigestFromAllPossibleTags(manifestName, svcToBuild string, buildInfo *model.BuildInfo, commit string) (string, error)
}

type registryImageCheckerInterface interface {
	GetImageTagWithDigest(string) (string, error)
}

type imageChecker struct {
	tagger   imageTaggerInterface
	cfg      oktetoBuilderConfigInterface
	registry registryImageCheckerInterface

	getImageSHA func(tag string, registry registryImageCheckerInterface) (string, error)
}

// newImageChecker returns a new image checker
func newImageChecker(cfg oktetoBuilderConfigInterface, registry registryImageCheckerInterface, tagger imageTaggerInterface) imageChecker {
	return imageChecker{
		tagger:   tagger,
		cfg:      cfg,
		registry: registry,

		getImageSHA: getImageSHA,
	}
}

// checkIfBuildHashIsBuilt returns if the buildhash is already built
// in case is built, the image with digest is also returned
// if not, image with digest is empty
func (ic imageChecker) checkIfBuildHashIsBuilt(manifestName, svcToBuild string, buildInfo *model.BuildInfo, buildHash string) (string, bool) {
	if buildHash == "" {
		return "", false
	}
	tagsToCheck := ic.tagger.getPossibleHashImages(manifestName, svcToBuild, buildHash)

	for _, tag := range tagsToCheck {
		imageWithDigest, err := ic.getImageSHA(tag, ic.registry)
		if err != nil {
			if oktetoErrors.IsNotFound(err) {
				continue
			}
			oktetoLog.Infof("could not check image %s: %s", tag, err)
			return "", false
		}
		return imageWithDigest, true
	}
	return "", false
}

func (ic imageChecker) getImageDigestFromAllPossibleTags(manifestName, svcToBuild string, buildInfo *model.BuildInfo, buildHash string) (string, error) {

	var possibleTags []string
	if !ic.cfg.IsOkteto() && shouldAddVolumeMounts(buildInfo) {
		possibleTags = []string{buildInfo.Image}
	} else if shouldAddVolumeMounts(buildInfo) {
		possibleTags = ic.tagger.getPossibleTags(manifestName, svcToBuild, buildHash)
	} else if shouldBuildFromDockerfile(buildInfo) && buildInfo.Image == "" {
		possibleTags = ic.tagger.getPossibleTags(manifestName, svcToBuild, buildHash)
	} else if buildInfo.Image != "" {
		possibleTags = []string{buildInfo.Image}
	}

	for _, tag := range possibleTags {
		imageWithDigest, err := ic.getImageSHA(tag, ic.registry)
		if err != nil {
			if oktetoErrors.IsNotFound(err) {
				continue
			}
			// return error if the registry doesn't send a not found error
			return "", fmt.Errorf("error checking image at registry %s: %v", tag, err)
		}
		return imageWithDigest, nil
	}
	return "", fmt.Errorf("images [%s] not found", strings.Join(possibleTags, ", "))
}

func getImageSHA(tag string, registry registryImageCheckerInterface) (string, error) {
	imageWithDigest, err := registry.GetImageTagWithDigest(tag)
	if err != nil {
		return "", fmt.Errorf("error checking image at registry %s: %w", tag, err)
	}
	return imageWithDigest, nil
}
