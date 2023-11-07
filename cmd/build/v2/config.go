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

package v2

import (
	"os"
	"strconv"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/afero"
)

const (
	// OktetoEnableSmartBuildEnvVar represents whether the feature flag to enable smart builds is enabled or not
	OktetoEnableSmartBuildEnvVar = "OKTETO_SMART_BUILDS_ENABLED"
)

type configRepositoryInterface interface {
	GetSHA() (string, error)
	IsClean() (bool, error)
	GetAnonymizedRepo() string
	GetTreeHash(string) (string, error)
}

type configRegistryInterface interface {
	HasGlobalPushAccess() (bool, error)
}

type oktetoBuilderConfig struct {
	hasGlobalAccess     bool
	isCleanProject      bool
	repository          configRepositoryInterface
	fs                  afero.Fs
	isOkteto            bool
	isSmartBuildsEnable bool
}

func getConfig(registry configRegistryInterface, gitRepo configRepositoryInterface) oktetoBuilderConfig {
	hasAccess, err := registry.HasGlobalPushAccess()
	if err != nil {
		oktetoLog.Infof("error trying to access globalPushAccess: %w", err)
	}

	isClean, err := gitRepo.IsClean()
	if err != nil {
		oktetoLog.Infof("error trying to get directory: %w", err)
	}

	return oktetoBuilderConfig{
		repository:          gitRepo,
		hasGlobalAccess:     hasAccess,
		isCleanProject:      isClean,
		fs:                  afero.NewOsFs(),
		isOkteto:            okteto.Context().IsOkteto,
		isSmartBuildsEnable: getIsSmartBuildEnabled(),
	}
}

func getIsSmartBuildEnabled() bool {
	enableSmartBuilds := true
	enableSmartBuildsStr := os.Getenv(OktetoEnableSmartBuildEnvVar)
	if enableSmartBuildsStr != "" {
		smartBuildEnabledBool, err := strconv.ParseBool(enableSmartBuildsStr)
		if err != nil {
			oktetoLog.Warning("feature flag %s received an invalid value; expected boolean. Smart builds will remain enabled by default", OktetoEnableSmartBuildEnvVar)
		} else {
			enableSmartBuilds = smartBuildEnabledBool
		}

	}

	return enableSmartBuilds
}

// IsOkteto checks if the context is an okteto managed context
func (oc oktetoBuilderConfig) IsOkteto() bool {
	return oc.isOkteto
}

// HasGlobalAccess checks if the user has access to global registry
func (oc oktetoBuilderConfig) HasGlobalAccess() bool {
	return oc.hasGlobalAccess
}

// IsCleanProject checks if the repository is clean(no changes over the last commit)
func (oc oktetoBuilderConfig) IsCleanProject() bool {
	return oc.isCleanProject
}

// GetGitCommit returns the commit sha of the repository
func (oc oktetoBuilderConfig) GetGitCommit() string {
	commitSHA, err := oc.repository.GetSHA()
	if err != nil {
		oktetoLog.Infof("could not get repository sha: %w", err)
	}
	return commitSHA
}

// GetAnonymizedRepo returns the repository url without credentials
func (oc oktetoBuilderConfig) GetAnonymizedRepo() string {
	return oc.repository.GetAnonymizedRepo()
}

func (oc oktetoBuilderConfig) GetBuildContextHash(buildInfo *model.BuildInfo) string {
	buildContext := buildInfo.Context
	treeHash, err := oc.repository.GetTreeHash(buildInfo.Context)
	if err != nil {
		oktetoLog.Info("error trying to get tree hash for build context '%s': %w", buildContext, err)
	}

	return getBuildHashFromGitHash(buildInfo, treeHash, "tree_hash")
}

func (oc oktetoBuilderConfig) IsSmartBuildsEnabled() bool {
	return oc.isSmartBuildsEnable
}
