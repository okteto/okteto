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
	"time"

	"github.com/okteto/okteto/pkg/build"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/spf13/afero"
)

type repositoryCommitRetriever interface {
	GetSHA() (string, error)
	GetLatestDirSHA(string) (string, error)
	GetDiffHash(string) (string, error)
}

type osWorkingDirGetter interface {
	Get() (string, error)
}

type serviceHasher struct {
	gitRepoCtrl repositoryCommitRetriever

	fs afero.Fs

	wdGetter osWorkingDirGetter

	serviceShaCache map[string]string

	getCurrentTimestampNano func() int64
	projectCommit           string

	ioCtrl *io.Controller

	// lock is a mutex to provide thread safety
	lock sync.RWMutex
}

func newServiceHasher(gitRepoCtrl repositoryCommitRetriever, fs afero.Fs, wdGetter osWorkingDirGetter, ioCtrl *io.Controller) *serviceHasher {
	return &serviceHasher{

		gitRepoCtrl:             gitRepoCtrl,
		serviceShaCache:         map[string]string{},
		fs:                      fs,
		getCurrentTimestampNano: time.Now().UnixNano,
		wdGetter:                wdGetter,
		ioCtrl:                  ioCtrl,
	}
}

// hashProjectCommit returns the hash of the repository's commit
func (sh *serviceHasher) hashProjectCommit(buildInfo *build.Info) (string, error) {
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
func (sh *serviceHasher) hashWithBuildContext(buildInfo *build.Info, service string) string {
	buildContext := buildInfo.Context
	if buildContext == "" {
		buildContext = "."
	}

	osWD, err := sh.wdGetter.Get()
	if err != nil {
		oktetoLog.Infof("could not get working directory to calculate hash for service %q: %s. Calculation of hash might not be correct", service, err)

	} else if !filepath.IsAbs(buildContext) {
		// Build context might be an absolute path (when coming from compose file) or a relative one (when coming from Okteto manifest)
		// If it is a relative path, we join it with the working directory to have the absolute path and pass it to the repo controller to calculate sha and diff
		// If it is an absolute path we should not do anything as it should point to the right build context
		buildContext = filepath.Join(osWD, buildContext)
	}

	sh.ioCtrl.Logger().Infof("working directory: %s", osWD)
	sh.ioCtrl.Logger().Infof("smart build context directory: %s", buildContext)
	
	// Check cache with read lock first
	sh.lock.RLock()
	if hash, ok := sh.serviceShaCache[service]; ok {
		sh.lock.RUnlock()
		return hash
	}
	sh.lock.RUnlock()
	
	// Not in cache, need to calculate - use write lock
	sh.lock.Lock()
	defer sh.lock.Unlock()
	
	// Double-check pattern: another goroutine might have calculated it while we were waiting for the lock
	if hash, ok := sh.serviceShaCache[service]; ok {
		return hash
	}
	
	// Calculate the hash
	errorGettingGitInfo := false
	dirCommit, err := sh.gitRepoCtrl.GetLatestDirSHA(buildContext)
	if err != nil {
		errorGettingGitInfo = true

		sh.ioCtrl.Logger().Infof("could not get build context sha: %s, generating a random one", err)
		// In case of error getting the dir commit, we just generate a random one, and it will rebuild the image
		dirCommit = sh.calculateRandomShaForService(service)
	}

	diffHash, err := sh.gitRepoCtrl.GetDiffHash(buildContext)
	if err != nil {
		errorGettingGitInfo = true
		sh.ioCtrl.Logger().Infof("could not get build context diff sha: %s, generating a random one", err)
		// In case of error getting the diff hash, we just generate a random one, and it will rebuild the image
		diffHash = sh.calculateRandomShaForService(service)
	}

	// This is to display just one single warning if any of the git operation fails. As we generate random sha
	// it will imply a new build of image, and we want to warn users
	if errorGettingGitInfo {
		sh.ioCtrl.Out().Warning("Smart builds cannot access git metadata, building image %q...", service)
	}

	hash := sh.hash(buildInfo, dirCommit, diffHash)
	sh.serviceShaCache[service] = hash
	return hash
}

// calculateRandomShaForService generates a random sha for the given service taking into account current timestamp
// in nanoseconds
func (sh *serviceHasher) calculateRandomShaForService(service string) string {
	key := fmt.Sprintf("%s-%d", service, sh.getCurrentTimestampNano())

	sha := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sha[:])
}

func (sh *serviceHasher) hash(buildInfo *build.Info, commitHash string, diff string) string {
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
	fmt.Fprintf(&b, "dockerfile_content:%s;", sh.getDockerfileContent(buildInfo.Context, buildInfo.Dockerfile))
	fmt.Fprintf(&b, "diff:%s;", diff)
	fmt.Fprintf(&b, "image:%s;", buildInfo.Image)

	hashFrom := b.String()
	sh.ioCtrl.Logger().Infof("hashing build info: %s", hashFrom)
	oktetoBuildHash := sha256.Sum256([]byte(hashFrom))
	return hex.EncodeToString(oktetoBuildHash[:])
}

// getDockerfileContent returns the content of the Dockerfile
func (sh *serviceHasher) getDockerfileContent(dockerfileContext, dockerfilePath string) string {
	content, err := afero.ReadFile(sh.fs, dockerfilePath)
	if err != nil {
		sh.ioCtrl.Logger().Infof("error trying to read Dockerfile on path '%s': %s", dockerfilePath, err)
		if errors.Is(err, os.ErrNotExist) {
			dockerfilePath = filepath.Join(dockerfileContext, dockerfilePath)
			content, err = afero.ReadFile(sh.fs, dockerfilePath)
			if err != nil {
				sh.ioCtrl.Logger().Infof("error trying to read Dockerfile: %s", err)
				return ""
			}
		}
	}
	encodedFile := sha256.Sum256(content)
	return hex.EncodeToString(encodedFile[:])
}
