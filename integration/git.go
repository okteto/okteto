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

package integration

import (
	"fmt"
	"log"
	"os"
	"os/exec"
)

// CloneGitRepo clones a repo from an URL
func CloneGitRepo(url string) error {
	log.Printf("cloning git repo %s", url)
	cmd := exec.Command("git", "clone", url)
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cloning git repo %s failed: %s - %w", url, string(o), err)
	}
	log.Printf("clone git repo %s success", url)
	return nil
}

// CloneGitRepoWithBranch clones a repo by its URL in an specific branch
func CloneGitRepoWithBranch(url, branch string) error {
	log.Printf("cloning git repo %s on branch %s", url, branch)
	cmd := exec.Command("git", "clone", "--branch", branch, url)
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cloning git repo %s failed: %s - %w", url, string(o), err)
	}
	log.Printf("clone git repo %s success", url)
	return nil
}

// DeleteGitRepo deletes the path passed as argument
func DeleteGitRepo(path string) error {
	log.Printf("delete git repo %s", path)
	err := os.RemoveAll(path)
	if err != nil {
		return fmt.Errorf("delete git repo %s failed: %w", path, err)
	}

	log.Printf("deleted git repo %s", path)
	return nil
}
