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
	"os"
	"strings"

	"github.com/okteto/okteto/pkg/env"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
)

const (
	// OktetoSmartBuildUsingContextEnvVar is the env var to enable smart builds using the build context instead of the project build
	OktetoSmartBuildUsingContextEnvVar = "OKTETO_SMART_BUILDS_USING_BUILD_CONTEXT"
)

type repositoryCommitRetriever interface {
	GetSHA() (string, error)
	GetLatestDirCommit(string) (string, error)
}

type serviceHasher struct {
	gitRepoCtrl repositoryCommitRetriever

	buildContextCache map[string]string
	projectCommit     string
}

func newServiceHasher(gitRepoCtrl repositoryCommitRetriever) *serviceHasher {
	return &serviceHasher{
		gitRepoCtrl:       gitRepoCtrl,
		buildContextCache: map[string]string{},
	}
}

// hashProjectCommit returns the hash of the repository's commit
func (sh *serviceHasher) hashProjectCommit(buildInfo *model.BuildInfo) string {
	if sh.projectCommit == "" {
		var err error
		sh.projectCommit, err = sh.gitRepoCtrl.GetSHA()
		if err != nil {
			oktetoLog.Infof("could not get repository sha: %w", err)
		}
	}
	return sh.hash(buildInfo, sh.projectCommit)
}

// hashBuildContext returns the hash of the service using its context tree hash
func (sh serviceHasher) hashBuildContext(buildInfo *model.BuildInfo) string {
	buildContext := buildInfo.Context
	if buildContext == "" {
		buildContext = "."
	}
	if _, ok := sh.buildContextCache[buildContext]; !ok {
		var err error
		sh.buildContextCache[buildContext], err = sh.gitRepoCtrl.GetLatestDirCommit(buildContext)
		if err != nil {
			oktetoLog.Info("error trying to get tree hash for build context '%s': %w", buildContext, err)
		}
	}

	return sh.hash(buildInfo, sh.buildContextCache[buildContext])
}

// hashService returns the hashed project commit by default. If smart-builds use the context it returns the hash of the service given its git tree hash
func (sh serviceHasher) hashService(buildInfo *model.BuildInfo) string {
	if env.LoadBoolean(OktetoSmartBuildUsingContextEnvVar) {
		return sh.hashBuildContext(buildInfo)
	}
	return sh.hashProjectCommit(buildInfo)
}

func (sh serviceHasher) hash(buildInfo *model.BuildInfo, commitHash string) string {
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

	fmt.Fprintf(&b, "commit:%s;", commitHash)
	fmt.Fprintf(&b, "target:%s;", buildInfo.Target)
	fmt.Fprintf(&b, "build_args:%s;", argsText)
	fmt.Fprintf(&b, "secrets:%s;", secretsText)
	fmt.Fprintf(&b, "context:%s;", buildInfo.Context)
	fmt.Fprintf(&b, "dockerfile:%s;", buildInfo.Dockerfile)
	fmt.Fprintf(&b, "dockerfile_content:%s;", getDockerfileContent(buildInfo.Dockerfile))
	fmt.Fprintf(&b, "image:%s;", buildInfo.Image)

	oktetoBuildHash := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(oktetoBuildHash[:])
}

func (sh serviceHasher) GetCommitHash(buildInfo *model.BuildInfo) string {
	if env.LoadBoolean(OktetoSmartBuildUsingContextEnvVar) {
		buildContext := buildInfo.Context
		if buildContext == "" {
			buildContext = "."
		}
		if commit, ok := sh.buildContextCache[buildContext]; ok {
			return commit
		}
		oktetoLog.Infof("could not find commit for build context '%s'", buildContext)
		return ""
	}
	return sh.projectCommit
}

// getDockerfileContent returns the content of the Dockerfile
func getDockerfileContent(dockerfilePath string) string {
	content, err := os.ReadFile(dockerfilePath)
	if err != nil {
		oktetoLog.Info("error trying to read Dockerfile: %w", err)
		return ""
	}
	encodedFile := sha256.Sum256(content)
	return hex.EncodeToString(encodedFile[:])
}
