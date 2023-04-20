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

	"github.com/google/go-containerregistry/pkg/name"
	oktetoLog "github.com/okteto/okteto/pkg/log"
)

const globalTestImage = "okteto.global/test"

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

type registryConfig interface {
	IsOktetoCluster() bool
	GetRegistryURL() string
}

// OktetoRegistry represents the registry
type OktetoRegistry struct {
	client    clientInterface
	imageCtrl ImageCtrl
	config    registryConfig
}

type OktetoImageReference struct {
	Registry string
	Repo     string
	Tag      string
	Image    string
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
	repository, _ := or.imageCtrl.GetRepoNameAndTag(repositoryWithTag)

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

func (or OktetoRegistry) IsGlobalRegistry(image string) bool {
	expandedImage := or.imageCtrl.expandImageRegistries(image)
	expandedGlobalImage := fmt.Sprintf("%s/%s", or.config.GetRegistryURL(), or.imageCtrl.config.GetGlobalNamespace())
	return strings.HasPrefix(expandedImage, expandedGlobalImage)
}

// GetImageTag returns the image tag to build for a given services
func (or OktetoRegistry) GetImageTag(image, service, namespace string) string {
	if or.config.GetRegistryURL() != "" {
		if or.IsOktetoRegistry(image) {
			return image
		}
		return fmt.Sprintf("%s/%s/%s:okteto", or.config.GetRegistryURL(), namespace, service)
	}
	imageWithoutTag, _ := or.imageCtrl.GetRepoNameAndTag(image)
	return fmt.Sprintf("%s:okteto", imageWithoutTag)
}

// GetImageReference returns the values to setup the image environment variables
func (OktetoRegistry) GetImageReference(image string) (OktetoImageReference, error) {
	ref, err := name.ParseReference(image)
	if err != nil {
		return OktetoImageReference{}, err
	}
	return OktetoImageReference{
		Registry: ref.Context().RegistryStr(),
		Repo:     ref.Context().RepositoryStr(),
		Tag:      ref.Identifier(),
		Image:    image,
	}, nil
}

// HasGlobalPushAccess checks if the user has push access to the global registry
func (or OktetoRegistry) HasGlobalPushAccess() (bool, error) {
	if !or.config.IsOktetoCluster() {
		return false, nil
	}
	image := or.imageCtrl.ExpandOktetoGlobalRegistry(globalTestImage)
	return or.client.HasPushAccess(image)
}

func (or OktetoRegistry) GetRegistryAndRepo(image string) (string, string) {
	return or.imageCtrl.GetRegistryAndRepo(image)
}

func (or OktetoRegistry) GetRepoNameAndTag(repo string) (string, string) {
	return or.imageCtrl.GetRepoNameAndTag(repo)
}
