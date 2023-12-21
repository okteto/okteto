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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/spf13/afero"
)

type repositoryCommitRetriever interface {
	GetSHA() (string, error)
	GetLatestDirCommit(string) (string, error)
	GetDiffHash(string) (string, error)
}

type serviceHasher struct {
	gitRepoCtrl repositoryCommitRetriever
	fs          afero.Fs

	buildContextCache map[string]string
	diffHashCache     map[string]string
	projectCommit     string

	// lock is a mutex to provide thread safety
	lock sync.RWMutex
}

func newServiceHasher(gitRepoCtrl repositoryCommitRetriever, fs afero.Fs) *serviceHasher {
	return &serviceHasher{
		gitRepoCtrl:       gitRepoCtrl,
		buildContextCache: map[string]string{},
		diffHashCache:     map[string]string{},
		fs:                fs,
	}
}

// hashProjectCommit returns the hash of the repository's commit
func (sh *serviceHasher) hashProjectCommit(buildInfo *model.BuildInfo) (string, error) {
	sh.lock.Lock()
	projectCommit := sh.projectCommit
	sh.lock.Unlock()
	if projectCommit == "" {
		var err error
		projectCommit, err = sh.gitRepoCtrl.GetSHA()
		if err != nil {
			return "", fmt.Errorf("could not get repository sha: %w", err)
		}
		sh.lock.Lock()
		sh.projectCommit = projectCommit
		sh.lock.Unlock()
	}
	return sh.hash(buildInfo, projectCommit, ""), nil
}

// hashBuildContext returns the hash of the service using its context tree hash
func (sh *serviceHasher) hashBuildContext(buildInfo *model.BuildInfo) (string, error) {
	buildContext := buildInfo.Context
	if buildContext == "" {
		buildContext = "."
	}
	if _, ok := sh.buildContextCache[buildContext]; !ok {
		var err error
		sh.buildContextCache[buildContext], err = sh.gitRepoCtrl.GetLatestDirCommit(buildContext)
		if err != nil {
			return "", fmt.Errorf("could not get build context sha: %w", err)
		}

		sh.diffHashCache[buildContext], err = sh.gitRepoCtrl.GetDiffHash(buildContext)
		if err != nil {
			return "", fmt.Errorf("could not get build context diff sha: %w", err)
		}
	}

	return sh.hash(buildInfo, sh.buildContextCache[buildContext], sh.diffHashCache[buildContext]), nil
}

func (sh *serviceHasher) hash(buildInfo *model.BuildInfo, commitHash string, diff string) string {
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
	fmt.Fprintf(&b, "dockerfile_content:%s;", sh.getDockerfileContent(buildInfo.Context, buildInfo.Dockerfile))
	fmt.Fprintf(&b, "diff:%s;", diff)
	fmt.Fprintf(&b, "image:%s;", buildInfo.Image)

	oktetoBuildHash := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(oktetoBuildHash[:])
}

// getDockerfileContent returns the content of the Dockerfile
func (sh *serviceHasher) getDockerfileContent(dockerfileContext, dockerfilePath string) string {
	content, err := afero.ReadFile(sh.fs, dockerfilePath)
	if err != nil {
		oktetoLog.Infof("error trying to read Dockerfile on path '%s': %s", dockerfilePath, err)
		if errors.Is(err, os.ErrNotExist) {
			dockerfilePath = filepath.Join(dockerfileContext, dockerfilePath)
			content, err = afero.ReadFile(sh.fs, dockerfilePath)
			if err != nil {
				oktetoLog.Infof("error trying to read Dockerfile: %s", err)
				return ""
			}
		}
	}
	encodedFile := sha256.Sum256(content)
	return hex.EncodeToString(encodedFile[:])
}

func (sh *serviceHasher) getBuildContextHashInCache(buildContext string) string {
	sh.lock.RLock()
	defer sh.lock.RUnlock()
	v, ok := sh.buildContextCache[buildContext]
	if !ok {
		return ""
	}
	return v
}

func (sh *serviceHasher) getProjectCommitHashInCache() string {
	sh.lock.RLock()
	defer sh.lock.RUnlock()
	return sh.projectCommit
}
