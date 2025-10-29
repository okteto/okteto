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
	"context"

	"github.com/okteto/okteto/cmd/build/v2/environment"
	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/spf13/afero"
)

// registryController is the interface to interact with registries
type registryController interface {
	GetDevImageFromGlobal(string) string
	GetImageTagWithDigest(string) (string, error)
	Clone(string, string) (string, error)
	IsGlobalRegistry(string) bool
	IsOktetoRegistry(string) bool
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
}

// CheckStrategy is the interface for the check strategy
type CheckStrategy interface {
	CheckServicesCache(ctx context.Context, manifestName string, buildManifest build.ManifestBuild, toBuildSvcs []string) ([]string, []string, error)
	GetImageDigestReferenceForServiceDeploy(manifestName, service string, buildInfo *build.Info) (string, error)
}

// Ctrl is the controller for smart builds
type Ctrl struct {
	CheckStrategy
	gitRepo            repositoryInterface
	registryController registryController
	ioCtrl             *io.Controller

	hasher hasherController
	config *Config
}

// NewSmartBuildCtrl creates a new smart build controller
func NewSmartBuildCtrl(
	repo repositoryInterface,
	registry registryController,
	fs afero.Fs,
	ioCtrl *io.Controller,
	wdGetter osWorkingDirGetter,
	smartBuildConfig *Config,
	tagger ImageTagger,
	imageCtrl registry.ImageCtrl,
	serviceEnvVarsHandler *environment.ServiceEnvVarsHandler,
	ns string,
	registryURL string) *Ctrl {
	hasher := newServiceHasher(repo, fs, wdGetter, ioCtrl)
	cloner := NewCloner(registry, ioCtrl)
	cacheChecker := NewRegistryCacheProbe(tagger, ns, registryURL, imageCtrl, registry, ioCtrl)

	var checkStrategy CheckStrategy
	if smartBuildConfig.isSequentialCheckStrategy {
		checkStrategy = NewSequentialCheckStrategy(tagger, hasher, cacheChecker, ioCtrl, serviceEnvVarsHandler, cloner)
	} else {
		// TODO: Implement parallel check strategy
		// For now, use sequential strategy as fallback since parallel is not implemented yet
		checkStrategy = NewSequentialCheckStrategy(tagger, hasher, cacheChecker, ioCtrl, serviceEnvVarsHandler, cloner)
	}

	return &Ctrl{
		gitRepo:            repo,
		config:             smartBuildConfig,
		hasher:             hasher,
		registryController: registry,
		ioCtrl:             ioCtrl,
		CheckStrategy:      checkStrategy,
	}
}

// IsEnabled returns true if smart builds are enabled, false otherwise
func (s *Ctrl) IsEnabled() bool {
	return s.config.isEnabled
}

// GetProjectHash returns the commit hash of the project
func (s *Ctrl) GetProjectHash(buildInfo *build.Info) (string, error) {
	s.ioCtrl.Logger().Debugf("getting project hash")
	return s.hasher.hashProjectCommit(buildInfo)
}

// GetBuildHash returns the hash of the build based on the env vars
func (s *Ctrl) GetBuildHash(buildInfo *build.Info, service string) string {
	s.ioCtrl.Logger().Debugf("getting hash based on the buildContext env var")
	s.ioCtrl.Logger().Info("getting hash using build context due to env var")
	return s.hasher.hashWithBuildContext(buildInfo, service)
}
