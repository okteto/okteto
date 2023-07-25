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

const buildPath = "okteto/build"

func TestBuildActionPipeline(t *testing.T) {
	e2e.SkipIfWindows(t)

	namespace := e2e.GetTestNamespace("buildaction", user)

	assert.NoError(t, executeCreateNamespaceAction(namespace))

	dockerfile, err := createDockerfile(t)
	assert.NoError(t, err)

	assert.NoError(t, executeBuildCommand(namespace, dockerfile))
	assert.NoError(t, executeDeleteNamespaceAction(namespace))
}

func createDockerfile(t *testing.T) (string, error) {
	dir := t.TempDir()
	log.Printf("created tempdir: %s", dir)
	dockerfilePath := filepath.Join(dir, "Dockerfile")
	dockerfileContent := []byte("FROM alpine")
	if err := os.WriteFile(dockerfilePath, dockerfileContent, 0600); err != nil {
		return "", err
	}
	return dockerfilePath, nil
}

func executeBuildCommand(namespace, dockerfile string) error {
	actionFolder := strings.Split(buildPath, "/")[1]
	actionRepo := fmt.Sprintf("%s%s.git", githubHTTPSURL, buildPath)
	if err := e2e.CloneGitRepoWithBranch(actionRepo, oktetoVersion); err != nil {
		if err := e2e.CloneGitRepo(actionRepo); err != nil {
			return err
		}
		log.Printf("cloned repo %s main branch\n", actionRepo)
	}
	defer e2e.DeleteGitRepo(actionFolder)
	command := fmt.Sprintf("%s/entrypoint.sh", actionFolder)

	args := []string{fmt.Sprintf("okteto.dev/%s:latest", namespace), dockerfile, filepath.Dir(dockerfile)}

	log.Printf("building image")
	cmd := exec.Command(command, args...)
	cmd.Env = os.Environ()
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", command, strings.Join(args, " "), string(o))
	}
	return nil
}
