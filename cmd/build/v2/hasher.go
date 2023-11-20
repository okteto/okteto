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
	"strconv"
	"strings"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
)

const (
	OktetoSmartBuildUsingContextEnvVar = "OKTETO_SMART_BUILDS_USING_BUILD_CONTEXT"
)

type repositoryCommitRetriever interface {
	GetSHA() (string, error)
	GetTreeHash(string) (string, error)
}

type serviceHasher struct {
	gitRepoCtrl repositoryCommitRetriever

	projectCommit string
}

func newServiceHasher(gitRepoCtrl repositoryCommitRetriever) *serviceHasher {
	return &serviceHasher{
		gitRepoCtrl: gitRepoCtrl,
	}
}

// HashProjectCommit hashes the
func (sh *serviceHasher) HashProjectCommit(buildInfo *model.BuildInfo) string {
	if sh.projectCommit == "" {
		var err error
		sh.projectCommit, err = sh.gitRepoCtrl.GetSHA()
		if err != nil {
			oktetoLog.Infof("could not get repository sha: %w", err)
		}
	}
	return sh.hash(buildInfo, "commit", sh.projectCommit)
}

func (sh serviceHasher) HashBuildContext(buildInfo *model.BuildInfo) string {
	buildContext := buildInfo.Context
	if buildContext == "" {
		buildContext = "."
	}
	treeHash, err := sh.gitRepoCtrl.GetTreeHash(buildContext)
	if err != nil {
		oktetoLog.Info("error trying to get tree hash for build context '%s': %w", buildContext, err)
	}
	return sh.hash(buildInfo, "tree_hash", treeHash)
}

func (sh serviceHasher) HashService(buildInfo *model.BuildInfo) string {
	if LoadBoolean(OktetoSmartBuildUsingContextEnvVar) {
		return sh.HashBuildContext(buildInfo)
	}
	return sh.HashProjectCommit(buildInfo)
}

func (sh serviceHasher) hash(buildInfo *model.BuildInfo, commitType, commitHash string) string {
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

	fmt.Fprintf(&b, "%s:%s;", commitType, commitHash)
	fmt.Fprintf(&b, "target:%s;", buildInfo.Target)
	fmt.Fprintf(&b, "build_args:%s;", argsText)
	fmt.Fprintf(&b, "secrets:%s;", secretsText)
	fmt.Fprintf(&b, "context:%s;", buildInfo.Context)
	fmt.Fprintf(&b, "dockerfile:%s;", buildInfo.Dockerfile)
	fmt.Fprintf(&b, "image:%s;", buildInfo.Image)

	oktetoBuildHash := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(oktetoBuildHash[:])
}

// LoadBoolean loads a boolean environment variable and returns it value
func LoadBoolean(k string) bool {
	v := os.Getenv(k)
	if v == "" {
		v = "false"
	}

	h, err := strconv.ParseBool(v)
	if err != nil {
		return false
	}

	return h
}
