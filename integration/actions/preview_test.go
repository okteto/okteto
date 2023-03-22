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
	"strings"
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/stretchr/testify/assert"
)

const (
	deployPreviewPath  = "okteto/deploy-preview"
	destroyPreviewPath = "okteto/destroy-preview"
)

func TestPreviewActions(t *testing.T) {
	integration.SkipIfWindows(t)
	namespace := integration.GetTestNamespace("PreviewActions", user)

	assert.NoError(t, executeDeployPreviewAction(namespace))
	assert.NoError(t, executeDestroyPreviewAction(namespace))
}

func executeDeployPreviewAction(namespace string) error {
	oktetoPath, err := integration.GetOktetoPath()
	if err != nil {
		return err
	}
	actionRepo := fmt.Sprintf("%s%s.git", githubHTTPSURL, deployPreviewPath)
	actionFolder := strings.Split(deployPreviewPath, "/")[1]
	log.Printf("cloning destroy path repository: %s", actionRepo)
	if err := integration.CloneGitRepoWithBranch(actionRepo, oktetoVersion); err != nil {
		if err := integration.CloneGitRepo(actionRepo); err != nil {
			return err
		}
		log.Printf("cloned repo %s main branch\n", actionRepo)
	}
	log.Printf("cloned repo %s \n", actionRepo)
	defer integration.DeleteGitRepo(actionFolder)

	log.Printf("Deploying preview %s", namespace)
	command := oktetoPath
	args := []string{"preview", "deploy", namespace, "--scope", "personal", "--branch", "master", "--repository", fmt.Sprintf("%s%s", githubHTTPSURL, pipelineRepo), "--wait"}
	cmd := exec.Command(command, args...)
	cmd.Env = os.Environ()
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", command, strings.Join(args, " "), string(o))
	}

	log.Printf("destroy preview output: \n%s\n", string(o))
	return nil
}

func executeDestroyPreviewAction(namespace string) error {
	actionRepo := fmt.Sprintf("%s%s.git", githubHTTPSURL, destroyPreviewPath)
	actionFolder := strings.Split(destroyPreviewPath, "/")[1]
	log.Printf("cloning destroy path repository: %s", actionRepo)
	if err := integration.CloneGitRepoWithBranch(actionRepo, oktetoVersion); err != nil {
		if err := integration.CloneGitRepo(actionRepo); err != nil {
			return err
		}
		log.Printf("cloned repo %s main branch\n", actionRepo)
	}
	log.Printf("cloned repo %s \n", actionRepo)
	defer integration.DeleteGitRepo(actionFolder)

	log.Printf("Deleting preview %s", namespace)
	command := "chmod"
	entrypointPath := fmt.Sprintf("%s/entrypoint.sh", actionFolder)
	args := []string{"+x", entrypointPath}
	cmd := exec.Command(command, args...)
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", command, strings.Join(args, " "), string(o))
	}

	args = []string{namespace}
	cmd = exec.Command(entrypointPath, args...)
	cmd.Env = os.Environ()
	o, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", entrypointPath, strings.Join(args, " "), string(o))
	}

	log.Printf("destroy preview output: \n%s\n", string(o))
	return nil
}
