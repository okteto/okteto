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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/afero"
)

type configRepositoryInterface interface {
	GetSHA() (string, error)
	IsClean() (bool, error)
}

type configRegistryInterface interface {
	HasGlobalPushAccess() (bool, error)
}

type oktetoBuilderConfig struct {
	hasGlobalAccess bool
	isCleanProject  bool
	repository      configRepositoryInterface
	fs              afero.Fs
	isOkteto        bool
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
		repository:      gitRepo,
		hasGlobalAccess: hasAccess,
		isCleanProject:  isClean,
		fs:              afero.NewOsFs(),
		isOkteto:        okteto.Context().IsOkteto,
	}
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

func (oc oktetoBuilderConfig) GetGitCommit() string {
	commitSHA, err := oc.repository.GetSHA()
	if err != nil {
		oktetoLog.Infof("could not get repository sha: %w", err)
	}
	return commitSHA
}

// GetBuildTag returns a sha hash of the build info and the commit sha
func (oc oktetoBuilderConfig) GetBuildHash(buildInfo *model.BuildInfo) string {
	commitSHA, err := oc.repository.GetSHA()
	if err != nil {
		return ""
	}
	text := oc.getTextToHash(buildInfo, commitSHA)
	buildHash := sha256.Sum256([]byte(text))
	return hex.EncodeToString(buildHash[:])
}

func (oc oktetoBuilderConfig) getTextToHash(buildInfo *model.BuildInfo, sha string) string {
	args := []string{}
	for _, arg := range buildInfo.Args {
		args = append(args, arg.String())
	}
	argsText := strings.Join(args, ";")

	secrets := []string{}
	for key, value := range buildInfo.Secrets {
		secrets = append(secrets, fmt.Sprintf("%s=%s", key, value))
	}
	secretsText := strings.Join(secrets, ";")

	// We use a builder to avoid allocations when building the string
	var b strings.Builder
	fmt.Fprintf(&b, "commit:%s;", sha)
	fmt.Fprintf(&b, "target:%s;", buildInfo.Target)
	fmt.Fprintf(&b, "build_args:%s;", argsText)
	fmt.Fprintf(&b, "secrets:%s;", secretsText)
	fmt.Fprintf(&b, "context:%s;", buildInfo.Context)
	fmt.Fprintf(&b, "dockerfile:%s;", buildInfo.Dockerfile)
	fmt.Fprintf(&b, "image:%s;", buildInfo.Image)
	return b.String()
}
