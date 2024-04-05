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

package smartbuild

import (
	"fmt"

	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/env"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/spf13/afero"
)

const (
	// OktetoEnableSmartBuildEnvVar represents whether the feature flag to enable smart builds is enabled or not
	OktetoEnableSmartBuildEnvVar = "OKTETO_SMART_BUILDS_ENABLED"
)

// registryController is the interface to interact with registries
type registryController interface {
	CloneGlobalImageToDev(string) (string, error)
	IsGlobalRegistry(string) bool
}

// repositoryInterface is the interface to interact with git repositories
type repositoryInterface interface {
	GetSHA() (string, error)
	GetLatestDirSHA(string) (string, error)
	GetDiffHash(string) (string, error)
}

type hasherController interface {
	hashProjectCommit(*build.Info) (string, error)
	hashWithBuildContext(*build.Info, string) string
	getServiceShaInCache(string) string
}

// Ctrl is the controller for smart builds
type Ctrl struct {
	gitRepo            repositoryInterface
	registryController registryController
	ioCtrl             *io.Controller

	hasher hasherController

	isEnabled bool
}

// NewSmartBuildCtrl creates a new smart build controller
func NewSmartBuildCtrl(repo repositoryInterface, registry registryController, fs afero.Fs, ioCtrl *io.Controller) *Ctrl {
	isEnabled := env.LoadBooleanOrDefault(OktetoEnableSmartBuildEnvVar, true)

	return &Ctrl{
		gitRepo:            repo,
		isEnabled:          isEnabled,
		hasher:             newServiceHasher(repo, fs),
		registryController: registry,
		ioCtrl:             ioCtrl,
	}
}

// IsEnabled returns true if smart builds are enabled, false otherwise
func (s *Ctrl) IsEnabled() bool {
	return s.isEnabled
}

// GetProjectHash returns the commit hash of the project
func (s *Ctrl) GetProjectHash(buildInfo *build.Info) (string, error) {
	s.ioCtrl.Logger().Debugf("getting project hash")
	return s.hasher.hashProjectCommit(buildInfo)
}

// GetServiceHash returns the hash of the service
func (s *Ctrl) GetServiceHash(buildInfo *build.Info, service string) string {
	s.ioCtrl.Logger().Debugf("getting service hash")
	return s.hasher.hashWithBuildContext(buildInfo, service)
}

// GetBuildHash returns the hash of the build based on the env vars
func (s *Ctrl) GetBuildHash(buildInfo *build.Info, service string) string {
	s.ioCtrl.Logger().Debugf("getting hash based on the buildContext env var")
	s.ioCtrl.Logger().Info("getting hash using build context due to env var")
	return s.hasher.hashWithBuildContext(buildInfo, service)
}

// CloneGlobalImageToDev clones the image from the global registry to the dev registry if needed
// if the built image belongs to global registry we clone it to the dev registry
// so that in can be used in dev containers (i.e. okteto up)
func (s *Ctrl) CloneGlobalImageToDev(image string) (string, error) {
	if s.registryController.IsGlobalRegistry(image) {
		s.ioCtrl.Logger().Debugf("Copying image '%s' from global to personal registry", image)
		devImage, err := s.registryController.CloneGlobalImageToDev(image)
		if err != nil {
			return "", fmt.Errorf("error cloning image '%s': %w", image, err)
		}
		return devImage, nil
	}
	s.ioCtrl.Logger().Debugf("Image '%s' is not in the global registry", image)
	return image, nil
}
