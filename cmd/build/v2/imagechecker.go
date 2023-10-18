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
	checkIfBuildHashIsBuilt(manifestName, svcToBuild string, commit string) (string, bool)
	getImageDigestReferenceForService(manifestName, svcToBuild string, buildInfo *model.BuildInfo, commit string) (string, error)
}

type registryImageCheckerInterface interface {
	GetImageTagWithDigest(string) (string, error)
}

type imageChecker struct {
	tagger   imageTaggerInterface
	cfg      oktetoBuilderConfigInterface
	registry registryImageCheckerInterface

	lookupReferenceWithDigest func(tag string, registry registryImageCheckerInterface) (string, error)
}

// newImageChecker returns a new image checker
func newImageChecker(cfg oktetoBuilderConfigInterface, registry registryImageCheckerInterface, tagger imageTaggerInterface) imageChecker {
	return imageChecker{
		tagger:   tagger,
		cfg:      cfg,
		registry: registry,

		lookupReferenceWithDigest: lookupReferenceWithDigest,
	}
}

// checkIfBuildHashIsBuilt returns if the buildhash is already built
// in case is built, the image with digest ([name]@sha256:[sha]) is returned
// if not, empty reference is returned
func (ic imageChecker) checkIfBuildHashIsBuilt(manifestName, svcToBuild string, buildHash string) (string, bool) {
	if buildHash == "" {
		return "", false
	}
	// [name]:[tag] being the tag the buildHash
	referencesToCheck := ic.tagger.getImageReferencesForTag(manifestName, svcToBuild, buildHash)

	for _, ref := range referencesToCheck {
		imageWithDigest, err := ic.lookupReferenceWithDigest(ref, ic.registry)
		if err != nil {
			if oktetoErrors.IsNotFound(err) {
				continue
			}
			oktetoLog.Infof("could not check image %s: %s", ref, err)
			return "", false
		}
		return imageWithDigest, true
	}
	return "", false
}

// getImageDigestReferenceForService returns the image reference with digest for the given service
// format: [name]@sha256:[digest]
func (ic imageChecker) getImageDigestReferenceForService(manifestName, svcToBuild string, buildInfo *model.BuildInfo, buildHash string) (string, error) {

	// get all possible references
	var possibleReferences []string
	if !ic.cfg.IsOkteto() && serviceHasVolumesToInclude(buildInfo) {
		possibleReferences = []string{buildInfo.Image}
	} else if serviceHasVolumesToInclude(buildInfo) {
		possibleReferences = ic.tagger.getImageReferencesForTagWithDefaults(manifestName, svcToBuild, buildHash)
	} else if serviceHasDockerfile(buildInfo) && buildInfo.Image == "" {
		possibleReferences = ic.tagger.getImageReferencesForTagWithDefaults(manifestName, svcToBuild, buildHash)
	} else if buildInfo.Image != "" {
		possibleReferences = []string{buildInfo.Image}
	}

	for _, ref := range possibleReferences {
		imageWithDigest, err := ic.lookupReferenceWithDigest(ref, ic.registry)
		if err != nil {
			if oktetoErrors.IsNotFound(err) {
				continue
			}
			// return error if the registry doesn't send a not found error
			return "", fmt.Errorf("error checking image at registry %s: %v", ref, err)
		}
		return imageWithDigest, nil
	}
	return "", fmt.Errorf("images [%s] not found", strings.Join(possibleReferences, ", "))
}

// lookupReferenceWithDigest returns the image reference with the digest format if found at the given registry.
// format output: [name]@sha265:[digest]
func lookupReferenceWithDigest(reference string, registry registryImageCheckerInterface) (string, error) {
	imageWithDigest, err := registry.GetImageTagWithDigest(reference)
	if err != nil {
		return "", fmt.Errorf("error checking image at registry %s: %w", reference, err)
	}
	return imageWithDigest, nil
}
