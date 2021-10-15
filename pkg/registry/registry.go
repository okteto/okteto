// Copyright 2021 The Okteto Authors
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
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	v1 "k8s.io/api/core/v1"
)

const (
	DockerRegistry = "https://registry.hub.docker.com"
)

type ImageInfo struct {
	Config *ConfigInfo `json:"config"`
}

type ConfigInfo struct {
	ExposedPorts *map[string]*interface{} `json:"ExposedPorts"`
}

// GetImageTagWithDigest returns the image tag digest
func GetImageTagWithDigest(imageTag string) (string, error) {
	if !okteto.IsOkteto() {
		return imageTag, nil
	}

	var err error
	expandedTag := imageTag
	expandedTag = ExpandOktetoDevRegistry(expandedTag)
	expandedTag = ExpandOktetoGlobalRegistry(expandedTag)
	username := okteto.Context().UserID
	u, err := url.Parse(okteto.Context().Registry)
	if err != nil {
		log.Infof("error parsing registry url: %s", err.Error())
		return imageTag, nil
	}
	u.Scheme = "https"
	c, err := NewRegistryClient(u.String(), username, okteto.Context().Token)
	if err != nil {
		log.Infof("error creating registry client: %s", err.Error())
		return imageTag, nil
	}

	repoURL, tag := GetRepoNameAndTag(expandedTag)
	index := strings.IndexRune(repoURL, '/')
	if index == -1 {
		log.Infof("malformed registry url: %s", repoURL)
		return imageTag, nil
	}
	repoName := repoURL[index+1:]
	digest, err := c.ManifestDigest(repoName, tag)
	if err != nil {
		if strings.Contains(err.Error(), "status=404") {
			return "", errors.ErrNotFound
		}
		return "", fmt.Errorf("error getting image tag digest: %s", err.Error())
	}
	return fmt.Sprintf("%s@%s", repoName, digest.String()), nil
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

// TransformOktetoDevToGlobalRegistry returns the tag pointing to the global registry
func TransformOktetoDevToGlobalRegistry(tag string) string {
	return strings.Replace(tag, okteto.DevRegistry, okteto.GlobalRegistry, 1)
}

// SplitRegistryAndImage returns image tag and the registry to push the image
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

func GetHiddenExposePorts(image string) []model.Port {
	exposedPorts := make([]model.Port, 0)

	image = ExpandOktetoDevRegistry(image)
	image = ExpandOktetoGlobalRegistry(image)
	username := okteto.Context().UserID
	token := okteto.Context().Token

	registry := getRegistryURL(image)
	if registry == DockerRegistry {
		username = ""
		token = ""
	}

	c, err := NewRegistryClient(registry, username, token)
	if err != nil {
		log.Infof("error creating registry client: %s", err.Error())
		return exposedPorts
	}

	_, repo := GetRegistryAndRepo(image)
	repoName, tag := GetRepoNameAndTag(repo)
	if !strings.Contains(repoName, "/") {
		repoName = fmt.Sprintf("library/%s", repoName)
	}

	digest, err := c.ManifestV2(repoName, tag)
	if err != nil {
		log.Infof("error getting digest of %s/%s: %s", repoName, tag, err.Error())
		return exposedPorts
	}

	response, err := c.DownloadBlob(repoName, digest.Config.Digest)
	if err != nil {
		log.Infof("error getting digest of %s/%s: %s", repoName, tag, err.Error())
		return exposedPorts
	}

	info := ImageInfo{Config: &ConfigInfo{}}
	decoder := json.NewDecoder(response)
	decoder.Decode(&info)

	if info.Config.ExposedPorts != nil {

		for port := range *info.Config.ExposedPorts {
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
		return DockerRegistry
	} else {
		if !strings.HasPrefix(registry, "https://") {
			registry = fmt.Sprintf("https://%s", registry)
		}
		return registry
	}
}

// IsGlobalRegistry returns true if the tag is short for global registry
func IsGlobalRegistry(tag string) bool {
	return strings.HasPrefix(tag, okteto.GlobalRegistry)
}

// IsDevRegistry returns true if the tag is short for namespace registry
func IsDevRegistry(tag string) bool {
	return strings.HasPrefix(tag, okteto.DevRegistry)
}

// IsDevRegistry returns true if the tag is short for registry at Okteto
func IsOktetoRegistry(tag string) bool {
	return IsDevRegistry(tag) || IsGlobalRegistry(tag)
}

// replaceRegistry returns tag with registry replaced by the given
func replaceRegistry(input, registryType, namespace string) string {
	return strings.Replace(input, registryType, fmt.Sprintf("%s/%s", okteto.Context().Registry, namespace), 1)
}

// IsImageAtGlobalRegistry returns true if the image is at the global registry already
func IsImageAtGlobalRegistry(image string) (ok bool) {
	if !IsOktetoRegistry(image) {
		return false
	}
	okCommit := os.Getenv("OKTETO_GIT_COMMIT")
	if okCommit != "" && strings.Contains(image, okCommit) {
		globalRegistryTag := image
		if IsDevRegistry(image) {
			globalRegistryTag = TransformOktetoDevToGlobalRegistry(image)
		}
		if _, err := GetImageTagWithDigest(globalRegistryTag); err == nil {
			log.Success("Skipping build: image is already build at the global registry")
			log.Hint("You can force the build by using the flag --no-cache")
			return true
		}
	}
	return false
}
