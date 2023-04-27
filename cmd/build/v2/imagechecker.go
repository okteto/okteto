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

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
)

type imageCheckerInterface interface {
	checkIfCommitHashIsBuilt(manifestName, svcToBuild string, buildInfo *model.BuildInfo) (string, bool)
	getImageDigestFromAllPossibleTags(manifestName, svcToBuild string, buildInfo *model.BuildInfo) (string, error)
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

func (ic imageChecker) checkIfCommitHashIsBuilt(manifestName, svcToBuild string, buildInfo *model.BuildInfo) (string, bool) {
	sha := ic.cfg.GetBuildHash(buildInfo)
	if sha == "" {
		return "", false
	}
	tagsToCheck := ic.tagger.getPossibleHashImages(manifestName, svcToBuild, sha)

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

func (ic imageChecker) getImageDigestFromAllPossibleTags(manifestName, svcToBuild string, buildInfo *model.BuildInfo) (string, error) {
	sha := ic.cfg.GetBuildHash(buildInfo)

	var possibleTags []string
	if shouldAddVolumeMounts(buildInfo) {
		possibleTags = ic.tagger.getPossibleTags(manifestName, svcToBuild, sha)
	} else if shouldBuildFromDockerfile(buildInfo) && buildInfo.Image == "" {
		possibleTags = ic.tagger.getPossibleTags(manifestName, svcToBuild, sha)
	} else {
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
	return "", fmt.Errorf("not found")
}

func getImageSHA(tag string, registry registryImageCheckerInterface) (string, error) {
	imageWithDigest, err := registry.GetImageTagWithDigest(tag)
	if err != nil {
		return "", fmt.Errorf("error checking image at registry %s: %w", tag, err)
	}
	return imageWithDigest, nil
}
