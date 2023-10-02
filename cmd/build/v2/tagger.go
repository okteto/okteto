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

	"github.com/okteto/okteto/pkg/okteto"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/format"
	"github.com/okteto/okteto/pkg/model"
)

type imageTaggerInterface interface {
	tag(manifestName, svcName string, b *model.BuildInfo, buildHash string) string
	getPossibleHashImages(manifestName, svcToBuildName, sha string) []string
	getPossibleTags(manifestName, svcToBuildName, sha string) []string
}

type imageTagger struct {
	cfg oktetoBuilderConfigInterface
}

type imageWithVolumesTagger struct {
	cfg oktetoBuilderConfigInterface
}

func getTargetRegistries() []string {
	registries := []string{}

	if okteto.IsOkteto() {
		registries = append(registries, constants.DevRegistry, constants.GlobalRegistry)
	}

	return registries
}

// newImageTagger returns a new image tagger
func newImageTagger(cfg oktetoBuilderConfigInterface) imageTagger {
	return imageTagger{
		cfg: cfg,
	}
}

// tag returns the full image tag for the build
func (i imageTagger) tag(manifestName, svcName string, b *model.BuildInfo, buildHash string) string {
	targetRegistry := constants.DevRegistry
	sha := ""
	if i.cfg.HasGlobalAccess() && i.cfg.IsCleanProject() {
		targetRegistry = constants.GlobalRegistry
		sha = buildHash
	}
	sanitizedName := format.ResourceK8sMetaString(manifestName)
	if shouldBuildFromDockerfile(b) && b.Image == "" {
		if sha != "" {
			return getImageFromTmpl(targetRegistry, sanitizedName, svcName, sha)
		}
		return getImageFromTmpl(targetRegistry, sanitizedName, svcName, model.OktetoDefaultImageTag)
	}
	return b.Image
}

// getPossibleHashImages returns all the possible images that can be built from a commit hash
func (i imageTagger) getPossibleHashImages(manifestName, svcToBuildName, sha string) []string {
	if sha == "" {
		return []string{}
	}

	// manifestName can be not sanitized when option name is used at deploy
	sanitizedName := format.ResourceK8sMetaString(manifestName)
	tagsToCheck := []string{}
	for _, targetRegistry := range getTargetRegistries() {
		tagsToCheck = append(tagsToCheck, getImageFromTmpl(targetRegistry, sanitizedName, svcToBuildName, sha))
	}
	return tagsToCheck
}

// getPossibleTags returns all the possible images that can be built (with and without hash)
func (i imageTagger) getPossibleTags(manifestName, svcToBuildName, sha string) []string {
	tags := i.getPossibleHashImages(manifestName, svcToBuildName, sha)
	sanitizedName := format.ResourceK8sMetaString(manifestName)
	for _, targetRegistry := range getTargetRegistries() {
		tags = append(tags, getImageFromTmpl(targetRegistry, sanitizedName, svcToBuildName, model.OktetoDefaultImageTag))
	}
	return tags
}

// newImageWithVolumesTagger returns a new image tagger
func newImageWithVolumesTagger(cfg oktetoBuilderConfigInterface) imageWithVolumesTagger {
	return imageWithVolumesTagger{
		cfg: cfg,
	}
}

// tag returns the full image tag for the build
func (i imageWithVolumesTagger) tag(manifestName, svcName string, _ *model.BuildInfo, buildHash string) string {
	targetRegistry := constants.DevRegistry
	sha := ""
	if i.cfg.HasGlobalAccess() && i.cfg.IsCleanProject() {
		targetRegistry = constants.GlobalRegistry
		sha = buildHash
	}
	sanitizedName := format.ResourceK8sMetaString(manifestName)
	if sha != "" {
		return getImageFromTmplWithVolumesAndSHA(targetRegistry, sanitizedName, svcName, sha)
	}
	return getImageFromTmpl(targetRegistry, sanitizedName, svcName, model.OktetoImageTagWithVolumes)
}

// getPossibleHashImages returns all the possible images that can be built from a commit hash
func (i imageWithVolumesTagger) getPossibleHashImages(manifestName, svcToBuildName, sha string) []string {
	if sha == "" {
		return []string{}
	}
	// manifestName can be not sanitized when option name is used at deploy
	sanitizedName := format.ResourceK8sMetaString(manifestName)
	tagsToCheck := []string{}
	for _, targetRegistry := range getTargetRegistries() {
		tagsToCheck = append(tagsToCheck, getImageFromTmplWithVolumesAndSHA(targetRegistry, sanitizedName, svcToBuildName, sha))
	}
	return tagsToCheck
}

// getPossibleTags returns all the possible images that can be built (with and without hash)
func (i imageWithVolumesTagger) getPossibleTags(manifestName, svcToBuildName, sha string) []string {
	tags := i.getPossibleHashImages(manifestName, svcToBuildName, sha)
	sanitizedName := format.ResourceK8sMetaString(manifestName)
	for _, targetRegistry := range getTargetRegistries() {
		tags = append(tags, getImageFromTmpl(targetRegistry, sanitizedName, svcToBuildName, model.OktetoImageTagWithVolumes))
	}
	return tags
}

// getImageFromTmpl returns the image name from the template of image tag
func getImageFromTmpl(targetRegistry, repoName, svcName, tag string) string {
	return fmt.Sprintf("%s/%s-%s:%s", targetRegistry, repoName, svcName, tag)
}

// getImageFromTmpl returns the image name from the template of image sha
func getImageFromTmplWithVolumesAndSHA(targetRegistry, repoName, svcName, sha string) string {
	return fmt.Sprintf("%s/%s-%s:%s-%s", targetRegistry, repoName, svcName, model.OktetoImageTagWithVolumes, sha)
}
