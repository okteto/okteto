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
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/repository"
)

type oktetoBuilderConfig struct {
	hasGlobalAccess bool
	isCleanProject  bool
	repository      repository.Repository
}

func getConfig(registry oktetoRegistryInterface, gitRepo repository.Repository) oktetoBuilderConfigInterface {
	hasAccess, err := registry.HasGlobalPushAccess()
	if err != nil {
		oktetoLog.Infof("error trying to access globalPushAccess: %w", err)
	}

	isClean, err := gitRepo.IsClean()
	if err != nil {
		oktetoLog.Infof("error trying to get directory: %w", err)
	}
	return oktetoBuilderConfig{
		repository:      gitRepo,
		hasGlobalAccess: hasAccess,
		isCleanProject:  isClean,
	}
}

// HasGlobalAccess checks if the user has access to global registry
func (oc oktetoBuilderConfig) HasGlobalAccess() bool {
	return oc.hasGlobalAccess
}

// IsCleanProject checks if the repository is clean(no changes over the last commit)
func (oc oktetoBuilderConfig) IsCleanProject() bool {
	return oc.isCleanProject
}

func (oc oktetoBuilderConfig) GetHash() string {
	sha, err := oc.repository.GetSHA()
	if err != nil {
		oktetoLog.Infof("could not get repository sha: %w", err)
	}
	return sha
}
