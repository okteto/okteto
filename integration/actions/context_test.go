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
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/assert"
)

const contextPath = "okteto/context"

func TestContextAction(t *testing.T) {
	integration.SkipIfWindows(t)
	if os.Getenv(model.OktetoSkipContextTestEnvVar) != "" {
		t.Skip("this test is not required because of 'OKTETO_SKIP_CONTEXT_TEST' env var")
		return
	}
	var remove bool
	if _, err := os.Stat(config.GetOktetoContextFolder()); err != nil {
		remove = true
	}
	assert.NoError(t, executeContextAction())

	if remove {
		assert.NoError(t, os.RemoveAll(config.GetOktetoContextFolder()))
	}

}

func executeContextAction() error {
	token := os.Getenv(model.OktetoTokenEnvVar)
	if token == "" {
		token = okteto.GetContext().Token
	}

	actionRepo := fmt.Sprintf("%s%s.git", githubHTTPSURL, contextPath)
	actionFolder := strings.Split(contextPath, "/")[1]
	log.Printf("cloning build action repository: %s", actionRepo)
	if err := integration.CloneGitRepoWithBranch(actionRepo, oktetoVersion); err != nil {
		if err := integration.CloneGitRepo(actionRepo); err != nil {
			return err
		}
		log.Printf("cloned repo %s main branch\n", actionRepo)
	}
	log.Printf("cloned repo %s \n", actionRepo)
	defer integration.DeleteGitRepo(actionFolder)

	oktetoURL := os.Getenv(model.OktetoURLEnvVar)
	if oktetoURL == "" {
		oktetoURL = okteto.CloudURL
	}
	log.Printf("login into %s", oktetoURL)
	command := fmt.Sprintf("%s/entrypoint.sh", actionFolder)
	args := []string{token, oktetoURL}
	cmd := exec.Command(command, args...)
	cmd.Env = os.Environ()
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", command, strings.Join(args, " "), string(o))
	}
	log.Printf("context output: \n%s\n", string(o))
	return nil
}
