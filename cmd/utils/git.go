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

package utils

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model/utils"
)

var (
	isOktetoSample     int64
	isOktetoSampleOnce sync.Once
)

// GetBranch returns the branch from a .git directory
func GetBranch(path string) (string, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return "", fmt.Errorf("failed to analyze git repo: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to infer the git repo's current branch: %w", err)
	}

	branch := head.Name()
	if !branch.IsBranch() {
		return "", fmt.Errorf("git repo is not on a valid branch")
	}

	name := strings.TrimPrefix(branch.String(), "refs/heads/")
	return name, nil
}

// GetRandomSHA returns a random sha generated in the fly
func GetRandomSHA() string {
	var letters = []rune("0123456789abcdef")
	b := make([]rune, 40)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			oktetoLog.Info("could not generate new int")
		}
		b[i] = letters[n.Int64()]
	}
	return string(b)
}

func IsOktetoRepo() bool {
	isOktetoSampleOnce.Do(func() {
		path, err := os.Getwd()
		if err != nil {
			oktetoLog.Infof("failed to get the current working directory in IsOktetoRepo: %v", err)
			return
		}
		repoUrl, err := utils.GetRepositoryURL(path)
		if err != nil {
			oktetoLog.Infof("failed to get repository url in IsOktetoRepo: %v", err)
			return
		}
		if isOktetoRepoFromURL(repoUrl) {
			atomic.StoreInt64(&isOktetoSample, 1)
		}
	})

	return atomic.LoadInt64(&isOktetoSample) == 1
}

func isOktetoRepoFromURL(repoUrl string) bool {
	endpoint, err := transport.NewEndpoint(repoUrl)
	if err != nil {
		oktetoLog.Infof("failed to get endpoint in isOktetoRepoFromURL: %v", err)
		return false
	}
	endpoint.Path = strings.TrimPrefix(endpoint.Path, "/")
	return strings.HasPrefix(endpoint.Path, "okteto/")
}
