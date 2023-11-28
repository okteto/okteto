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
	"strconv"
	"strings"

	containerv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/okteto/okteto/pkg/constants"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	apiv1 "k8s.io/api/core/v1"
)

type ImageMetadata struct {
	Image   string
	CMD     []string
	Workdir string
	Ports   []Port
	Envs    []string
}

type Port struct {
	Protocol      apiv1.Protocol
	ContainerPort int32
}

func (Port) GetHostPort() int32            { return 0 }
func (p Port) GetContainerPort() int32     { return p.ContainerPort }
func (p Port) GetProtocol() apiv1.Protocol { return p.Protocol }

type ImageCtrl struct {
	config           imageConfig
	registryReplacer Replacer
}

type imageConfig interface {
	IsOktetoCluster() bool
	GetGlobalNamespace() string
	GetNamespace() string
	GetRegistryURL() string
}

func NewImageCtrl(config imageConfig) ImageCtrl {
	return ImageCtrl{
		config:           config,
		registryReplacer: NewRegistryReplacer(config),
	}
}

func NewImageCtrlFromContext(config imageConfig) ImageCtrl {
	return ImageCtrl{
		config:           config,
		registryReplacer: NewRegistryReplacer(config),
	}
}

func (ic ImageCtrl) expandImageRegistries(image string) string {
	if ic.config.IsOktetoCluster() {
		image = ic.ExpandOktetoDevRegistry(image)
		image = ic.ExpandOktetoGlobalRegistry(image)
	}
	return image
}

// ExpandOktetoGlobalRegistry translates okteto.global
func (ic ImageCtrl) ExpandOktetoGlobalRegistry(tag string) string {
	globalNamespace := constants.DefaultGlobalNamespace
	if ic.config.GetGlobalNamespace() != "" {
		globalNamespace = ic.config.GetGlobalNamespace()
	}
	return ic.registryReplacer.Replace(tag, constants.GlobalRegistry, globalNamespace)
}

// ExpandOktetoDevRegistry translates okteto.dev
func (ic ImageCtrl) ExpandOktetoDevRegistry(tag string) string {
	return ic.registryReplacer.Replace(tag, constants.DevRegistry, ic.config.GetNamespace())
}

// GetRegistryAndRepo returns image tag and the registry to push the image
func (ImageCtrl) GetRegistryAndRepo(tag string) (string, string) {
	var imageTag string
	registryTag := "docker.io"
	splittedImage := strings.Split(tag, "/")

	if len(splittedImage) == 1 {
		imageTag = splittedImage[0]
	} else if len(splittedImage) == 2 { //nolint:gomnd
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
func (ImageCtrl) GetRepoNameAndTag(image string) (string, string) {
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

func (ImageCtrl) getExposedPortsFromCfg(cfg *containerv1.ConfigFile) []Port {
	var result []Port
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

func GetDevTagFromGlobal(image string) string {
	if !strings.HasPrefix(image, constants.GlobalRegistry) {
		return ""
	}

	// separate image reference and tag eg: okteto.dev/image:tag
	reference, _, found := strings.Cut(image, "@sha256")
	if !found {
		reference, _, found = strings.Cut(image, ":")
		if !found {
			return ""
		}
	}

	devReference := strings.Replace(reference, constants.GlobalRegistry, constants.DevRegistry, 1)
	if devReference == reference {
		return ""
	}
	return fmt.Sprintf("%s:%s", devReference, model.OktetoDefaultImageTag)
}
