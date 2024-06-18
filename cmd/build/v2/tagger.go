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
	"regexp"
	"strings"

	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/format"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/registry"
)

var extendedImageRegex = regexp.MustCompile(`^[a-zA-Z0-9-_.]+\/[a-zA-Z0-9-_.]+\/([a-zA-Z0-9-_.]+):[a-zA-Z0-9-_.]+$`)

type imageTaggerInterface interface {
	getServiceDevImageReference(manifestName, svcName string, b *build.Info) string
	getImageReferencesForTag(manifestName, svcToBuildName, tag string) []string
	getImageReferencesForDeploy(manifestName, svcToBuildName string) []string
	getGlobalTagFromDevIfNeccesary(tags, namespace, registryURL, buildHash string, ic registry.ImageCtrl) string
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
getServiceDevImageReference returns the image reference [name]:[tag] for the given service.

When service image is set on manifest, this is the returned one.

Inferred tag is constructed using the following:
[name] is the combination of the targetRegistry, manifestName and serviceName
[tag] it is the default okteto tag "okteto".
*/
func (it imageTagger) getServiceDevImageReference(manifestName, svcName string, b *build.Info) string {
	// when b.Image is set or services does not have dockerfile then no infer reference and return what is set on the manifest
	if b.Image != "" || !serviceHasDockerfile(b) {
		return b.Image
	}

	// build the image reference based on context and buildInfo
	targetRegistry := constants.DevRegistry
	sanitizedName := format.ResourceK8sMetaString(manifestName)
	return useReferenceTemplate(targetRegistry, sanitizedName, svcName, model.OktetoDefaultImageTag)
}

func (it imageTagger) getGlobalTagFromDevIfNeccesary(tags, namespace, registryURL, buildHash string, ic registry.ImageCtrl) string {
	if !it.cfg.HasGlobalAccess() || !it.smartBuildController.IsEnabled() || buildHash == "" {
		return ""
	}
	tagList := strings.Split(tags, ",")
	globalWithHash := ""
	for _, tag := range tagList {
		expandedTag := ic.ExpandOktetoDevRegistry(tag)
		matches := extendedImageRegex.FindStringSubmatch(expandedTag)
		regexGroupFullNameRepo := 2
		if len(matches) != regexGroupFullNameRepo {
			continue
		}
		name := matches[1]
		if strings.HasPrefix(expandedTag, fmt.Sprintf("%s/%s/", registryURL, namespace)) {
			if globalWithHash != "" {
				globalWithHash += ","
			}
			newImage := fmt.Sprintf("%s/%s:%s", constants.GlobalRegistry, name, buildHash)
			globalWithHash += ic.ExpandOktetoGlobalRegistry(newImage)
		}
	}
	return globalWithHash
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

// getImageReferencesForDeploy returns the list of images references for a service when deploying it. In case of deploy,
// we only have to check if the image is present with the okteto tag. We don't check anything related to the hash
func (imageTagger) getImageReferencesForDeploy(manifestName, svcToBuildName string) []string {
	sanitizedName := format.ResourceK8sMetaString(manifestName)
	imageReferences := []string{useReferenceTemplate(constants.DevRegistry, sanitizedName, svcToBuildName, model.OktetoDefaultImageTag)}

	return imageReferences
}

// useReferenceTemplate returns the image reference with the given parameters [name]:[tag]
func useReferenceTemplate(targetRegistry, repoName, svcName, tag string) string {
	return fmt.Sprintf("%s/%s-%s:%s", targetRegistry, repoName, svcName, tag)
}
