// Copyright 2022 The Okteto Authors
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
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"k8s.io/utils/pointer"
)

var isOktetoSample *bool

func GetBranch(ctx context.Context, path string) (string, error) {
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

func GetGitCommit(ctx context.Context, path string) (string, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return "", fmt.Errorf("failed to analyze git repo: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to infer the git repo's current branch: %w", err)
	}

	hash := head.Hash()

	return hash.String(), nil
}

func IsCleanDirectory(ctx context.Context, path string) (bool, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return false, fmt.Errorf("failed to analyze git repo: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return false, fmt.Errorf("failed to infer the git repo's current branch: %w", err)
	}

	status, err := worktree.Status()
	if err != nil {
		return false, fmt.Errorf("failed to infer the git repo's status: %w", err)
	}

	return status.IsClean(), nil
}

func GetRandomSHA(ctx context.Context, path string) string {
	var letters = []rune("0123456789abcdef")
	b := make([]rune, 40)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func IsOktetoRepo() bool {
	if isOktetoSample == nil {
		path, err := os.Getwd()
		if err != nil {
			oktetoLog.Infof("failed to get the current working directory in IsOktetoRepo: %v", err)
			isOktetoSample = pointer.BoolPtr(false)
			return false
		}
		repoUrl, err := model.GetRepositoryURL(path)
		if err != nil {
			oktetoLog.Infof("failed to get repository url in IsOktetoRepo: %v", err)
			isOktetoSample = pointer.BoolPtr(false)
			return false
		}
		isOktetoSample = pointer.BoolPtr(isOktetoRepoFromURL(repoUrl))
	}
	return *isOktetoSample
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
