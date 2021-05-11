// Copyright 2020 The Okteto Authors
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
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/k8s/namespaces"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
)

type ImageInfo struct {
	Config *ConfigInfo `json:"config"`
}

type ConfigInfo struct {
	ExposedPorts *map[string]*interface{} `json:"ExposedPorts"`
}

//GetImageTagWithDigest returns the image tag digest
func GetImageTagWithDigest(ctx context.Context, namespace, imageTag string) (string, error) {
	registryURL, err := okteto.GetRegistry()
	if err != nil {
		if err != errors.ErrNotLogged {
			log.Infof("error accessing to okteto registry: %s", err.Error())
		}
		return imageTag, nil
	}

	expandedTag, err := ExpandOktetoDevRegistry(ctx, namespace, imageTag)
	if err != nil {
		log.Infof("error expanding okteto registry: %s", err.Error())
		return imageTag, nil
	}
	if !strings.HasPrefix(expandedTag, registryURL) {
		return imageTag, nil
	}
	username := okteto.GetUserID()
	token, err := okteto.GetToken()
	if err != nil {
		log.Infof("error getting token: %s", err.Error())
		return imageTag, nil
	}
	u, err := url.Parse(registryURL)
	if err != nil {
		log.Infof("error parsing registry url: %s", err.Error())
		return imageTag, nil
	}
	u.Scheme = "https"
	c, err := NewRegistryClient(u.String(), username, token.Token)
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

//ExpandOktetoDevRegistry translates okteto.dev
func ExpandOktetoDevRegistry(ctx context.Context, namespace, tag string) (string, error) {
	if !strings.HasPrefix(tag, okteto.DevRegistry) {
		return tag, nil
	}

	c, _, err := client.GetLocal()
	if err != nil {
		return "", fmt.Errorf("failed to load your local Kubeconfig: %s", err)
	}

	if namespace == "" {
		namespace = client.GetContextNamespace("")
	}

	n, err := namespaces.Get(ctx, namespace, c)
	if err != nil {
		return "", fmt.Errorf("failed to get your current namespace '%s': %s", namespace, err.Error())
	}
	if !namespaces.IsOktetoNamespace(n) {
		return "", fmt.Errorf("cannot use the okteto.dev container registry: your current namespace '%s' is not managed by okteto", namespace)
	}

	oktetoRegistryURL, err := okteto.GetRegistry()
	if err != nil {
		return "", fmt.Errorf("cannot use the okteto.dev container registry: unable to get okteto registry url: %s", err)
	}

	tag = strings.Replace(tag, okteto.DevRegistry, fmt.Sprintf("%s/%s", oktetoRegistryURL, namespace), 1)
	return tag, nil
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

func GetHiddenExposePorts(ctx context.Context, namespace, image string) []int32 {
	exposedPorts := make([]int32, 0)
	var err error
	var username string
	var token string
	if strings.HasPrefix(image, okteto.DevRegistry) {
		image, err = ExpandOktetoDevRegistry(ctx, namespace, image)
		if err != nil {
			log.Infof("Could not expand okteto dev registry: %s", err.Error())
		}
		username = okteto.GetUserID()
		okToken, err := okteto.GetToken()
		if err != nil {
			log.Infof("Could not expand okteto dev registry: %s", err.Error())
			return exposedPorts
		}
		token = okToken.Token
	}

	registry := getRegistryURL(ctx, namespace, image)

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
				exposedPorts = append(exposedPorts, int32(portInt))
			}
		}
	}
	return exposedPorts
}

func getRegistryURL(ctx context.Context, namespace, image string) string {
	registry, _ := GetRegistryAndRepo(image)
	if registry == "docker.io" {
		return "https://registry.hub.docker.com"
	} else {
		if !strings.HasPrefix(registry, "https://") {
			registry = fmt.Sprintf("https://%s", registry)
		}
		return registry
	}
}
