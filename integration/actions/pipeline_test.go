//go:build actions
// +build actions

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

package actions

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/assert"
)

const (
	deployPipelinePath  = "okteto/pipeline"
	destroyPipelinePath = "okteto/destroy-pipeline"

	githubUrl = "https://github.com"

	pipelineRepo    = "okteto/movies"
	pipelineRepoURL = "git@github.com:okteto/movies.git"
	pipelineFolder  = "movies"
)

func TestPipelineActions(t *testing.T) {
	integration.SkipIfWindows(t)

	namespace := getTestNamespace()

	assert.NoError(t, executeCreateNamespaceAction(namespace))
	assert.NoError(t, executeDeployPipelineAction(t, namespace))
	assert.NoError(t, executeDestroyPipelineAction(namespace))
	assert.NoError(t, executeDeleteNamespaceAction(namespace))
}

func executeDeployPipelineAction(t *testing.T, namespace string) error {
	actionRepo := fmt.Sprintf("%s%s.git", githubHttpsUrl, deployPipelinePath)
	actionFolder := strings.Split(deployPipelinePath, "/")[1]
	log.Printf("cloning pipeline repository: %s", actionRepo)
	err := integration.CloneGitRepoWithBranch(actionRepo, "master")
	if err != nil {
		return err
	}
	log.Printf("cloned repo %s \n", actionRepo)
	defer integration.DeleteGitRepo(actionFolder)

	t.Setenv(model.GithubRepositoryEnvVar, pipelineRepo)
	t.Setenv(model.GithubRefEnvVar, "master")
	t.Setenv(model.GithubServerURLEnvVar, githubUrl)

	log.Printf("deploying pipeline %s", namespace)
	command := fmt.Sprintf("%s/entrypoint.sh", actionFolder)
	args := []string{"movies", namespace}

	cmd := exec.Command(command, args...)
	cmd.Env = os.Environ()
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", command, strings.Join(args, " "), string(o))
	}
	log.Printf("Deploy pipeline output: \n%s\n", string(o))

	oktetoClient, err := okteto.NewOktetoClient()
	if err != nil {
		return err
	}
	okteto.Context().Namespace = namespace
	pipeline, err := oktetoClient.GetPipelineByName(context.Background(), "movies")
	if err != nil || pipeline == nil {
		return fmt.Errorf("Could not get deployment %s", namespace)
	}
	return nil
}

func executeDestroyPipelineAction(namespace string) error {
	actionRepo := fmt.Sprintf("%s%s.git", githubHttpsUrl, destroyPipelinePath)
	actionFolder := strings.Split(destroyPipelinePath, "/")[1]
	log.Printf("cloning destroy pipeline repository: %s", actionRepo)
	if err := integration.CloneGitRepo(actionRepo); err != nil {
		return err
	}
	log.Printf("cloned repo %s \n", actionRepo)
	defer integration.DeleteGitRepo(actionFolder)

	log.Printf("Deleting pipeline %s", namespace)
	command := fmt.Sprintf("%s/entrypoint.sh", actionFolder)
	args := []string{"movies", namespace}
	cmd := exec.Command(command, args...)
	cmd.Env = os.Environ()
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", command, strings.Join(args, " "), string(o))
	}

	log.Printf("destroy pipeline output: \n%s\n", string(o))
	return nil
}
