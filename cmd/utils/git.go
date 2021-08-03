// Copyright 2021 The Okteto Authors
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
	"strings"

	"github.com/go-git/go-git/v5"
)

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
