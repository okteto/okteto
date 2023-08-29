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
	GetServerNameOverride() string
	GetContextName() string
	GetExternalRegistryCredentials(registryHost string) (string, string, error)
}

// OktetoRegistry represents the registry
type OktetoRegistry struct {
	client    clientInterface
	imageCtrl ImageCtrl
	config    configInterface
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
	envs := cfgFile.Config.Env

	return ImageMetadata{
		Image:   image,
		CMD:     cmd,
		Workdir: workdir,
		Ports:   ports,
		Envs:    envs,
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

// GetRegistryAndRepo returns image and registry of a given image
func (or OktetoRegistry) GetRegistryAndRepo(image string) (string, string) {
	return or.imageCtrl.GetRegistryAndRepo(image)
}

// GetRepoNameAndTag returns the repo name and tag of a given image
func (or OktetoRegistry) GetRepoNameAndTag(repo string) (string, string) {
	return or.imageCtrl.GetRepoNameAndTag(repo)
}

// CloneGlobalImageToDev clones an image from the global registry to the dev registry
func (or OktetoRegistry) CloneGlobalImageToDev(imageWithDigest, tag string) (string, error) {
	// parse the image URI to extract registry and repository name
	reg, repositoryWithTag := or.imageCtrl.GetRegistryAndRepo(imageWithDigest)
	repo, _ := or.imageCtrl.GetRepoNameAndTag(repositoryWithTag)

	globalNamespacePrefix := fmt.Sprintf("%s/", or.imageCtrl.config.GetGlobalNamespace())

	// this function returns an error if invoked for an image that is not in the global registry
	if !strings.HasPrefix(repo, globalNamespacePrefix) {
		return "", fmt.Errorf("image repository '%s' is not in the global registry", repo)
	}

	// forging a new image URI in the dev registry, using the same repo name and tag as the global image
	personalNamespacePrefix := fmt.Sprintf("%s/", or.config.GetNamespace())
	devRepo := strings.Replace(repo, globalNamespacePrefix, personalNamespacePrefix, 1)
	devImage := fmt.Sprintf("%s/%s:%s", reg, devRepo, tag)

	newRef, err := name.ParseReference(devImage)
	if err != nil {
		return "", err
	}

	descriptor, err := or.client.GetDescriptor(imageWithDigest)
	if err != nil {
		return "", err
	}

	i, err := descriptor.Image()
	if err != nil {
		return "", err
	}

	// writing the new image to the registry
	err = or.client.Write(newRef, i)
	if err != nil {
		return "", err
	}

	return devImage, nil
}
