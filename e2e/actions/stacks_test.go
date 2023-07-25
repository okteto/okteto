//go:build actions
// +build actions

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

package actions

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okteto/okteto/e2e"
	"github.com/stretchr/testify/assert"
)

const (
	deployStackPath  = "okteto/deploy-stack"
	destroyStackPath = "okteto/destroy-stack"

	stackFile = `
name: test
services:
  app:
    image: nginx
    ports:
    - 8080:80
`
)

func TestStacksActions(t *testing.T) {
	e2e.SkipIfWindows(t)

	namespace := e2e.GetTestNamespace("stackaction", user)

	assert.NoError(t, executeCreateNamespaceAction(namespace))

	file, err := createStackFile(t)
	assert.NoError(t, err)

	assert.NoError(t, executeDeployStackAction(namespace, file))
	assert.NoError(t, executeDestroyStackAction(namespace, file))
	assert.NoError(t, executeDeleteNamespaceAction(namespace))
}

func executeDeployStackAction(namespace, filePath string) error {

	actionRepo := fmt.Sprintf("%s%s.git", githubHTTPSURL, deployStackPath)
	actionFolder := strings.Split(deployStackPath, "/")[1]
	log.Printf("cloning pipeline repository: %s", actionRepo)
	if err := e2e.CloneGitRepoWithBranch(actionRepo, oktetoVersion); err != nil {
		if err := e2e.CloneGitRepo(actionRepo); err != nil {
			return err
		}
		log.Printf("cloned repo %s main branch\n", actionRepo)
	}
	log.Printf("cloned repo %s \n", actionRepo)
	defer e2e.DeleteGitRepo(actionFolder)

	log.Printf("creating namespace %s", namespace)
	command := fmt.Sprintf("%s/entrypoint.sh", actionFolder)
	args := []string{namespace, "", "", filePath}

	cmd := exec.Command(command, args...)
	cmd.Env = os.Environ()
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", command, strings.Join(args, " "), string(o))
	}

	return nil
}

func executeDestroyStackAction(namespace, filePath string) error {
	actionRepo := fmt.Sprintf("%s%s.git", githubHTTPSURL, destroyStackPath)
	actionFolder := strings.Split(destroyStackPath, "/")[1]
	log.Printf("cloning destroy path repository: %s", actionRepo)
	if err := e2e.CloneGitRepoWithBranch(actionRepo, oktetoVersion); err != nil {
		if err := e2e.CloneGitRepo(actionRepo); err != nil {
			return err
		}
		log.Printf("cloned repo %s main branch\n", actionRepo)
	}
	log.Printf("cloned repo %s \n", actionRepo)
	defer e2e.DeleteGitRepo(actionFolder)

	log.Printf("Deleting compose %s", namespace)
	command := fmt.Sprintf("%s/entrypoint.sh", actionFolder)
	args := []string{namespace, "", filePath}
	cmd := exec.Command(command, args...)
	cmd.Env = os.Environ()
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", command, strings.Join(args, " "), string(o))
	}

	log.Printf("destroy compose output: \n%s\n", string(o))
	return nil
}

func createStackFile(t *testing.T) (string, error) {
	dir := t.TempDir()
	log.Printf("created tempdir: %s", dir)
	filePath := filepath.Join(dir, "okteto-stack.yaml")
	if err := os.WriteFile(filePath, []byte(stackFile), 0600); err != nil {
		return "", err
	}
	return filePath, nil
}
