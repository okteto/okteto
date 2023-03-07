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
	"crypto/x509"
	"fmt"
	"strings"

	oktetoLog "github.com/okteto/okteto/pkg/log"
)

// OktetoRegistryInterface represents the interface to interact with the registry
type OktetoRegistryInterface interface {
	GetImageTagWithDigest(image string) (string, error)
	GetImageMetadata(image string) (ImageMetadata, error)
	IsOktetoRegistry(image string) bool
	GetImageTag(image, service, namespace string) string
}

type configInterface interface {
	IsOktetoCluster() bool
	GetGlobalNamespace() string
	GetNamespace() string
	GetRegistryURL() string
	GetUserID() string
	GetToken() string
	IsInsecureSkipTLSVerifyPolicy() bool
	GetContextCertificate() (*x509.Certificate, error)
}

// OktetoRegistry represents the
type OktetoRegistry struct {
	client    clientInterface
	imageCtrl imageCtrl
	config    configInterface
}

func NewOktetoRegistry(config configInterface) OktetoRegistry {
	return OktetoRegistry{
		client:    newOktetoRegistryClient(config),
		imageCtrl: NewImageCtrl(config),
		config:    config,
	}
}

func (or OktetoRegistry) GetImageTagWithDigest(image string) (string, error) {
	expandedImage := or.imageCtrl.expandImageRegistries(image)

	registry, repositoryWithTag := or.imageCtrl.GetRegistryAndRepo(expandedImage)
	repository, _ := or.imageCtrl.getRepoNameAndTag(repositoryWithTag)

	digest, err := or.client.GetDigest(expandedImage)
	if err != nil {
		return "", fmt.Errorf("error getting image tag with digest: %w", err)
	}

	imageTag := fmt.Sprintf("%s/%s@%s", registry, repository, digest)
	oktetoLog.Debugf("image with digest: %s", imageTag)
	return imageTag, nil
}

func (or OktetoRegistry) GetImageMetadata(image string) (ImageMetadata, error) {
	image, err := or.GetImageTagWithDigest(image)
	if err != nil {
		return ImageMetadata{}, fmt.Errorf("error getting image metadata: %w", err)
	}
	cfgFile, err := or.client.GetImageConfig(image)
	if err != nil {
		return ImageMetadata{}, fmt.Errorf("error getting image metadata: %w", err)
	}
	ports := or.imageCtrl.getExposedPortsFromCfg(cfgFile)
	workdir := cfgFile.Config.WorkingDir
	cmd := cfgFile.Config.Cmd

	return ImageMetadata{
		Image:   image,
		CMD:     cmd,
		Workdir: workdir,
		Ports:   ports,
	}, nil
}

// IsOktetoRegistry returns if an image tag is pointing to the okteto registry
func (or OktetoRegistry) IsOktetoRegistry(image string) bool {
	expandedImage := or.imageCtrl.expandImageRegistries(image)
	return or.config.IsOktetoCluster() && strings.HasPrefix(expandedImage, or.config.GetRegistryURL())
}

// GetImageTag returns the image tag to build for a given services
func (or OktetoRegistry) GetImageTag(image, service, namespace string) string {
	if or.config.GetRegistryURL() != "" {
		if or.IsOktetoRegistry(image) {
			return image
		}
		return fmt.Sprintf("%s/%s/%s:okteto", or.config.GetRegistryURL(), namespace, service)
	}
	imageWithoutTag, _ := or.imageCtrl.getRepoNameAndTag(image)
	return fmt.Sprintf("%s:okteto", imageWithoutTag)
}
