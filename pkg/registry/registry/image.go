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

	containerv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/okteto/okteto/pkg/constants"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	apiv1 "k8s.io/api/core/v1"
)

type ImageMetadata struct {
	Image   string
	CMD     []string
	Workdir string
	Ports   []Port
}

type Port struct {
	ContainerPort int32
	Protocol      apiv1.Protocol
}

func (p Port) GetHostPort() int32          { return 0 }
func (p Port) GetContainerPort() int32     { return p.ContainerPort }
func (p Port) GetProtocol() apiv1.Protocol { return p.Protocol }

type imageCtrl struct {
	config configInterface
}

func newImageCtrl(config configInterface) imageCtrl {
	return imageCtrl{
		config: config,
	}
}

func (ic imageCtrl) expandImageRegistries(image string) string {
	if ic.config.IsOktetoCluster() {
		image = ic.expandOktetoDevRegistry(image)
		image = ic.expandOktetoGlobalRegistry(image)
	}
	return image
}

// ExpandOktetoGlobalRegistry translates okteto.global
func (ic imageCtrl) expandOktetoGlobalRegistry(tag string) string {
	globalNamespace := constants.DefaultGlobalNamespace
	if ic.config.GetGlobalNamespace() != "" {
		globalNamespace = ic.config.GetGlobalNamespace()
	}
	return ic.replaceRegistry(tag, constants.GlobalRegistry, globalNamespace)
}

// ExpandOktetoDevRegistry translates okteto.dev
func (ic imageCtrl) expandOktetoDevRegistry(tag string) string {
	return ic.replaceRegistry(tag, constants.DevRegistry, ic.config.GetNamespace())
}

// replaceRegistry replaces the short registry url with the okteto registry url
func (ic imageCtrl) replaceRegistry(input, registryType, namespace string) string {
	// Check if the registryType is the start of the sentence or has a whitespace before it
	var re = regexp.MustCompile(fmt.Sprintf(`(^|\s)(%s)`, registryType))
	if re.MatchString(input) {
		return strings.Replace(input, registryType, fmt.Sprintf("%s/%s", ic.config.GetRegistryURL(), namespace), 1)
	}
	return input
}

// GetRegistryAndRepo returns image tag and the registry to push the image
func (imageCtrl) getRegistryAndRepo(tag string) (string, string) {
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

// GetRepoNameAndTag returns the image repo and the tag separated
func (imageCtrl) getRepoNameAndTag(image string) (string, string) {
	var domain, remainder string
	i := strings.IndexRune(image, '@')
	if i != -1 {
		return image[:i], image[i+1:]
	}
	i = strings.IndexRune(image, '/')
	if i == -1 || (!strings.ContainsAny(image[:i], ".:") && image[:i] != "localhost") {
		domain, remainder = "", image
	} else {
		domain, remainder = image[:i], image[i+1:]
	}
	i = strings.LastIndex(remainder, ":")
	if i == -1 {
		return image, "latest"
	}
	if domain == "" {
		return remainder[:i], remainder[i+1:]
	}
	return fmt.Sprintf("%s/%s", domain, remainder[:i]), remainder[i+1:]
}

func (imageCtrl) getExposedPortsFromCfg(cfg *containerv1.ConfigFile) []Port {
	result := []Port{}
	if cfg.Config.ExposedPorts == nil {
		return result
	}
	for port := range cfg.Config.ExposedPorts {
		slashIndx := strings.Index(port, "/")
		if slashIndx != -1 {
			port = port[:slashIndx]
			portInt, err := strconv.ParseInt(port, 10, 32)
			if err != nil {
				oktetoLog.Debugf("could not parse exposed port %s: %w", port, err)
				continue
			}
			result = append(result, Port{ContainerPort: int32(portInt), Protocol: apiv1.ProtocolTCP})
		}
	}
	return result
}
