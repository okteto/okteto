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

// OktetoRegistry runs the build of an image
type OktetoRegistry struct{}

// NewOktetoRegistry creates a okteto registry
func NewOktetoRegistry() *OktetoRegistry {
	return &OktetoRegistry{}
}

// ImageConfig is the struct of the information that can be inferred from an image
type ImageConfig struct {
	CMD          []string
	Workdir      string
	ExposedPorts []int
}

type ImageMetadata struct {
	Image string
	Ports []model.Port
}

// GetImageTagWithDigest returns the image tag digest
func (*OktetoRegistry) GetImageTagWithDigest(imageTag string) (string, error) {
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

// GetImageConfigFromImage gets information from the image
func GetImageConfigFromImage(imageRef string) (*ImageConfig, error) {
	imageConfig := &ImageConfig{
		CMD:          []string{},
		ExposedPorts: []int{},
	}

	imageRef = ExpandOktetoDevRegistry(imageRef)
	imageRef = ExpandOktetoGlobalRegistry(imageRef)

	image, err := imageForReference(imageRef)
	if err != nil {
		return nil, err
	}

	configFile, err := image.ConfigFile()
	if err != nil {
		return nil, err
	}

	if configFile.Config.ExposedPorts != nil {
		for port := range configFile.Config.ExposedPorts {
			slashIndx := strings.Index(port, "/")
			if slashIndx != -1 {
				port = port[:slashIndx]
				portInt, err := strconv.Atoi(port)
				if err != nil {
					continue
				}
				imageConfig.ExposedPorts = append(imageConfig.ExposedPorts, portInt)

			}
		}
	}
	if configFile.Config.WorkingDir != "" {
		imageConfig.Workdir = configFile.Config.WorkingDir
	}

	if len(configFile.Config.Cmd) > 0 {
		imageConfig.CMD = configFile.Config.Cmd
	}
	return imageConfig, nil
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

// GetImageMetadata returns the ports exposed at the image
func GetImageMetadata(imageRef string) *ImageMetadata {
	result := &ImageMetadata{}
	result.Image = imageRef
	result.Ports = make([]model.Port, 0)

	imageRef = ExpandOktetoDevRegistry(imageRef)
	imageRef = ExpandOktetoGlobalRegistry(imageRef)

	image, err := imageForReference(imageRef)
	if err != nil {
		oktetoLog.Debugf("error in GetImageMetadata.imageForReference: %s", err.Error())
		return result
	}

	digest, err := image.Digest()
	if err != nil {
		oktetoLog.Debugf("error in GetImageMetadata.Digest: %s", err.Error())
		return result
	}
	registry, tag := GetRegistryAndRepo(imageRef)
	repository, _ := GetRepoNameAndTag(tag)
	result.Image = fmt.Sprintf("%s/%s@%s", registry, repository, digest.String())

	configFile, err := image.ConfigFile()
	if err != nil {
		oktetoLog.Debugf("error in GetImageMetadata.ConfigFile: %s", err.Error())
		return result
	}
	if configFile.Config.ExposedPorts != nil {
		for port := range configFile.Config.ExposedPorts {
			slashIndx := strings.Index(port, "/")
			if slashIndx != -1 {
				port = port[:slashIndx]
				portInt, err := strconv.ParseInt(port, 10, 32)
				if err != nil {
					continue
				}
				result.Ports = append(result.Ports, model.Port{ContainerPort: int32(portInt), Protocol: v1.ProtocolTCP})
			}
		}
	}
	return result
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
	return IsDevRegistry(tag) || IsGlobalRegistry(tag) || (okteto.IsOkteto() && strings.HasPrefix(tag, okteto.Context().Registry))
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
