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

	"github.com/okteto/okteto/integration"
	"github.com/stretchr/testify/assert"
)

const (
	testPath = "okteto/test"
)

func TestOktetoTestActions(t *testing.T) {
	integration.SkipIfWindows(t)
	namespace := integration.GetTestNamespace(t.Name())

	assert.NoError(t, executeCreateNamespaceAction(namespace))

	oktetoManifest, err := createOktetoTestManifest(t)
	assert.NoError(t, err)

	assert.NoError(t, executeOktetoTestAction(namespace, oktetoManifest))
}

func executeOktetoTestAction(namespace, oktetoManifest string) error {
	actionRepo := fmt.Sprintf("%s%s.git", githubHTTPSURL, testPath)
	actionFolder := strings.Split(testPath, "/")[1]
	log.Printf("cloning destroy path repository: %s", actionRepo)
	if err := integration.CloneGitRepoWithBranch(actionRepo, oktetoVersion); err != nil {
		if err := integration.CloneGitRepo(actionRepo); err != nil {
			return err
		}
		log.Printf("cloned repo %s main branch\n", actionRepo)
	}
	log.Printf("cloned repo %s \n", actionRepo)
	defer integration.DeleteGitRepo(actionFolder)

	builder := exec.Command("go", "build", "-o", "test", "./cmd/main.go")
	builder.Dir = actionFolder
	builder.Env = os.Environ()
	o, err := builder.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go build: %s", string(o))
	}

	testBinary, err := filepath.Abs(filepath.Join(actionFolder, "test"))
	if err != nil {
		return fmt.Errorf("filepath.Abs: %s", err)
	}

	command := exec.Command(testBinary, "", namespace, "", "", "", "", "", "", "")
	command.Env = os.Environ()
	command.Dir = filepath.Dir(oktetoManifest)
	o, err = command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("test: %s", string(o))
	}

	log.Printf("destroy preview output: \n%s\n", string(o))
	return nil
}

func createOktetoTestManifest(t *testing.T) (string, error) {
	manifest := `
test:
  unit:
    context: .
    image: alpine
    commands:
      - echo "OK unit"
  integration:
    context: .
    image: alpine
    commands:
      - echo "OK integration"
      - echo "OK" > coverage.txt
      - mkdir -p reports && echo "OK" > reports/additional-coverage.txt
    artifacts:
      - coverage.txt
      - reports
  e2e:
    context: .
    image: alpine
    commands:
      - echo "OK e2e"
`
	dir := t.TempDir()
	log.Printf("created tempdir: %s", dir)
	manifestPath := filepath.Join(dir, "okteto.yml")
	manifestContent := []byte(manifest)
	if err := os.WriteFile(manifestPath, manifestContent, 0600); err != nil {
		return "", err
	}
	return manifestPath, nil
}
