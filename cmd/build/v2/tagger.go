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

	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/format"
	"github.com/okteto/okteto/pkg/model"
)

type imageTaggerInterface interface {
	getServiceImageReference(manifestName, svcName string, b *build.Info, buildHash string) string
	getImageReferencesForTag(manifestName, svcToBuildName, tag string) []string
	getImageReferencesForTagWithDefaults(manifestName, svcToBuildName, tag string) []string
}

type smartBuildController interface {
	IsEnabled() bool
}

// imageTagger implements an imageTaggerInterface with no volume mounts
type imageTagger struct {
	cfg                  oktetoBuilderConfigInterface
	smartBuildController smartBuildController
}

func getTargetRegistries(isOkteto bool) []string {
	registries := []string{}

	if isOkteto {
		registries = append(registries, constants.DevRegistry, constants.GlobalRegistry)
	}

	return registries
}

// newImageTagger returns an instance of imageTagger with the given config
func newImageTagger(cfg oktetoBuilderConfigInterface, sbc smartBuildController) imageTagger {
	return imageTagger{
		cfg:                  cfg,
		smartBuildController: sbc,
	}
}

/*
getServiceImageReference returns the image reference [name]:[tag] for the given service.

When service image is set on manifest, this is one returned.

Inferred tag is constructed using the following:
[name] is the combination of the tarjetRegistry, manifestName and serviceName
[tag] its either the buildHash or the default okteto tag "okteto"
*/
func (i imageTagger) getServiceImageReference(manifestName, svcName string, b *build.Info, buildHash string) string {
	// when b.Image is set or services does not have dockerfile then no infer reference and return what is set on the manifest
	if b.Image != "" || !serviceHasDockerfile(b) {
		return b.Image
	}

	// build the image reference based on context and buildInfo
	targetRegistry := constants.DevRegistry
	tag := ""
	if i.cfg.HasGlobalAccess() && i.smartBuildController.IsEnabled() {
		if i.cfg.IsCleanProject() {
			targetRegistry = constants.GlobalRegistry
		}
		tag = buildHash
	}
	sanitizedName := format.ResourceK8sMetaString(manifestName)
	if tag != "" {
		return useReferenceTemplate(targetRegistry, sanitizedName, svcName, tag)
	}
	return useReferenceTemplate(targetRegistry, sanitizedName, svcName, model.OktetoDefaultImageTag)
}

// getImageReferencesForTag returns all the possible images references that can be used for build with the given tag
func (it imageTagger) getImageReferencesForTag(manifestName, svcToBuildName, tag string) []string {
	if tag == "" {
		return []string{}
	}

	// manifestName can be not sanitized when option name is used at deploy
	sanitizedName := format.ResourceK8sMetaString(manifestName)
	referencesToCheck := []string{}

	for _, targetRegistry := range getTargetRegistries(it.cfg.IsOkteto()) {
		referencesToCheck = append(referencesToCheck, useReferenceTemplate(targetRegistry, sanitizedName, svcToBuildName, tag))
	}
	return referencesToCheck
}

// getImageReferencesForTagWithDefaults returns all the possible image references for a given service, options include the given tag and the default okteto tag
func (i imageTagger) getImageReferencesForTagWithDefaults(manifestName, svcToBuildName, tag string) []string {
	var imageReferences []string
	if i.smartBuildController.IsEnabled() {
		imageReferences = append(imageReferences, i.getImageReferencesForTag(manifestName, svcToBuildName, tag)...)
	}

	imageReferences = append(imageReferences, i.getImageReferencesForTag(manifestName, svcToBuildName, model.OktetoDefaultImageTag)...)

	return imageReferences
}

// imageTaggerWithVolumes represent an imageTaggerInterface with an reference tag with volume mounts
type imagerTaggerWithVolumes struct {
	cfg                  oktetoBuilderConfigInterface
	smartBuildController smartBuildController
}

// newImageWithVolumesTagger returns a new image tagger
func newImageWithVolumesTagger(cfg oktetoBuilderConfigInterface, sbc smartBuildController) imagerTaggerWithVolumes {
	return imagerTaggerWithVolumes{
		cfg:                  cfg,
		smartBuildController: sbc,
	}
}

// getServiceImageReference returns the full image tag for the build
func (i imagerTaggerWithVolumes) getServiceImageReference(manifestName, svcName string, _ *build.Info, buildHash string) string {

	targetRegistry := constants.DevRegistry
	tag := ""
	if i.cfg.HasGlobalAccess() && i.smartBuildController.IsEnabled() {
		if i.cfg.IsCleanProject() {
			targetRegistry = constants.GlobalRegistry
		}
		tag = buildHash
	}
	sanitizedName := format.ResourceK8sMetaString(manifestName)
	if tag != "" {
		return useReferenceTemplateWithVolumes(targetRegistry, sanitizedName, svcName, tag)
	}
	return useReferenceTemplate(targetRegistry, sanitizedName, svcName, model.OktetoImageTagWithVolumes)
}

// getImageReferencesForTag returns all the possible images that can be built from a commit hash
func (i imagerTaggerWithVolumes) getImageReferencesForTag(manifestName, svcToBuildName, tag string) []string {
	if tag == "" {
		return []string{}
	}
	// manifestName can be not sanitized when option name is used at deploy
	sanitizedName := format.ResourceK8sMetaString(manifestName)
	tagsToCheck := []string{}
	for _, targetRegistry := range getTargetRegistries(i.cfg.IsOkteto()) {
		tagsToCheck = append(tagsToCheck, useReferenceTemplateWithVolumes(targetRegistry, sanitizedName, svcToBuildName, tag))
	}
	return tagsToCheck
}

// getImageReferencesForTagWithDefaults returns all the possible images that can be built (with and without hash)
func (i imagerTaggerWithVolumes) getImageReferencesForTagWithDefaults(manifestName, svcToBuildName, tag string) []string {
	tags := i.getImageReferencesForTag(manifestName, svcToBuildName, tag)
	sanitizedName := format.ResourceK8sMetaString(manifestName)
	for _, targetRegistry := range getTargetRegistries(i.cfg.IsOkteto()) {
		tags = append(tags, useReferenceTemplate(targetRegistry, sanitizedName, svcToBuildName, model.OktetoImageTagWithVolumes))
	}
	return tags
}

// useReferenceTemplate returns the image reference with the given parameters [name]:[tag]
func useReferenceTemplate(targetRegistry, repoName, svcName, tag string) string {
	return fmt.Sprintf("%s/%s-%s:%s", targetRegistry, repoName, svcName, tag)
}

// useReferenceTemplateWithVolumes returns the image reference from the template of image with volume mounts [name]:okteto-with-volume-mounts-[tag]
func useReferenceTemplateWithVolumes(targetRegistry, repoName, svcName, tag string) string {
	return fmt.Sprintf("%s/%s-%s:%s-%s", targetRegistry, repoName, svcName, model.OktetoImageTagWithVolumes, tag)
}
