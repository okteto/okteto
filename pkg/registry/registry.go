// Copyright 2022 The Okteto Authors
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

package registry

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	v1 "k8s.io/api/core/v1"
)

const (
	dockerRegistry = "https://registry.hub.docker.com"
)

// GetImageTagWithDigest returns the image tag digest
func GetImageTagWithDigest(imageTag string) (string, error) {
	reference := imageTag

	if okteto.IsOkteto() {
		reference = ExpandOktetoDevRegistry(reference)
		reference = ExpandOktetoGlobalRegistry(reference)
	}

	registry, image := GetRegistryAndRepo(reference)
	repository, _ := GetRepoNameAndTag(image)

	digest, err := digestForReference(reference)
	if err != nil {
		oktetoLog.Debugf("error: %s", err.Error())
		if strings.Contains(err.Error(), "MANIFEST_UNKNOWN") {
			return "", oktetoErrors.ErrNotFound
		}
		return "", fmt.Errorf("error getting image tag digest: %s", err.Error())
	}
	imageTag = fmt.Sprintf("%s/%s@%s", registry, repository, digest)
	oktetoLog.Debugf("image with digest: %s", imageTag)
	return imageTag, nil
}

// ExpandOktetoGlobalRegistry translates okteto.global
func ExpandOktetoGlobalRegistry(tag string) string {
	globalNamespace := okteto.DefaultGlobalNamespace
	if okteto.Context().GlobalNamespace != "" {
		globalNamespace = okteto.Context().GlobalNamespace
	}
	return replaceRegistry(tag, okteto.GlobalRegistry, globalNamespace)
}

// ExpandOktetoDevRegistry translates okteto.dev
func ExpandOktetoDevRegistry(tag string) string {
	return replaceRegistry(tag, okteto.DevRegistry, okteto.Context().Namespace)
}

// GetRegistryAndRepo returns image tag and the registry to push the image
func GetRegistryAndRepo(tag string) (string, string) {
	var imageTag string
	registryTag := "docker.io"
	splittedImage := strings.Split(tag, "/")

	if len(splittedImage) == 1 {
		imageTag = splittedImage[0]
	} else if len(splittedImage) == 2 {
		if strings.Contains(splittedImage[0], ".") {
			return splittedImage[0], splittedImage[1]
		}
		imageTag = strings.Join(splittedImage[len(splittedImage)-2:], "/")
	} else {
		imageTag = strings.Join(splittedImage[len(splittedImage)-2:], "/")
		registryTag = strings.Join(splittedImage[:len(splittedImage)-2], "/")
	}
	return registryTag, imageTag
}

// GetHiddenExposePorts returns the ports exposed at the image
func GetHiddenExposePorts(image string) []model.Port {
	exposedPorts := make([]model.Port, 0)

	image = ExpandOktetoDevRegistry(image)
	image = ExpandOktetoGlobalRegistry(image)

	config, err := configForReference(image)
	if err != nil {
		return exposedPorts
	}
	if config.ExposedPorts != nil {
		for port := range config.ExposedPorts {
			if strings.Contains(port, "/") {
				port = port[:strings.Index(port, "/")]
				portInt, err := strconv.Atoi(port)
				if err != nil {
					continue
				}
				exposedPorts = append(exposedPorts, model.Port{ContainerPort: int32(portInt), Protocol: v1.ProtocolTCP})
			}
		}
	}
	return exposedPorts
}

func getRegistryURL(image string) string {
	registry, _ := GetRegistryAndRepo(image)
	if registry == "docker.io" {
		return dockerRegistry
	}
	if !strings.HasPrefix(registry, "https://") {
		registry = fmt.Sprintf("https://%s", registry)
	}
	return registry

}

// IsGlobalRegistry returns if an image tag is pointing to the global okteto registry
func IsGlobalRegistry(tag string) bool {
	return strings.HasPrefix(tag, okteto.GlobalRegistry)
}

// IsDevRegistry returns if an image tag is pointing to the dev okteto registry
func IsDevRegistry(tag string) bool {
	return strings.HasPrefix(tag, okteto.DevRegistry)
}

// IsOktetoRegistry returns if an image tag is pointing to the okteto registry
func IsOktetoRegistry(tag string) bool {
	return IsDevRegistry(tag) || IsGlobalRegistry(tag)
}

// replaceRegistry replaces the short registry url with the okteto registry url
func replaceRegistry(input, registryType, namespace string) string {
	// Check if the registryType is the start of the sentence or has a whitespace before it
	var re = regexp.MustCompile(fmt.Sprintf(`(^|\s)(%s)`, registryType))
	if re.MatchString(input) {
		return strings.Replace(input, registryType, fmt.Sprintf("%s/%s", okteto.Context().Registry, namespace), 1)
	}
	return input
}
