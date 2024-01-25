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
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/assert"
)

const (
	createNamespacePath = "okteto/create-namespace"
	namespacePath       = "okteto/namespace"
	deleteNamespacePath = "okteto/delete-namespace"
)

func TestNamespaceActionsPipeline(t *testing.T) {
	integration.SkipIfWindows(t)

	namespace := integration.GetTestNamespace("namespaceaction", user)

	assert.NoError(t, executeCreateNamespaceAction(namespace))
	assert.NoError(t, executeChangeNamespaceAction(namespace))
	assert.NoError(t, executeDeleteNamespaceAction(namespace))
}

func executeCreateNamespaceAction(namespace string) error {
	okteto.CurrentStore = nil
	actionRepo := fmt.Sprintf("%s%s.git", githubHTTPSURL, createNamespacePath)
	actionFolder := strings.Split(createNamespacePath, "/")[1]
	log.Printf("cloning create namespace repository: %s", actionRepo)
	if err := integration.CloneGitRepoWithBranch(actionRepo, oktetoVersion); err != nil {
		if err := integration.CloneGitRepo(actionRepo); err != nil {
			return err
		}
		log.Printf("cloned repo %s main branch\n", actionRepo)
	}

	defer integration.DeleteGitRepo(actionFolder)

	log.Printf("creating namespace %s", namespace)
	command := fmt.Sprintf("%s/entrypoint.sh", actionFolder)

	args := []string{namespace}
	cmd := exec.Command(command, args...)
	cmd.Env = os.Environ()
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", command, strings.Join(args, " "), string(o))
	}

	log.Printf("create namespace output: \n%s\n", string(o))
	n := okteto.GetContext().Namespace
	if namespace != n {
		return fmt.Errorf("current namespace is %s, expected %s", n, namespace)
	}

	return nil
}

func executeChangeNamespaceAction(namespace string) error {
	okteto.CurrentStore = nil
	actionRepo := fmt.Sprintf("%s%s.git", githubHTTPSURL, namespacePath)
	actionFolder := strings.Split(namespacePath, "/")[1]
	log.Printf("cloning changing namespace repository: %s", actionRepo)
	if err := integration.CloneGitRepoWithBranch(actionRepo, oktetoVersion); err != nil {
		if err := integration.CloneGitRepo(actionRepo); err != nil {
			return err
		}
		log.Printf("cloned repo %s main branch\n", actionRepo)
	}
	log.Printf("cloned repo %s \n", actionRepo)
	defer integration.DeleteGitRepo(actionFolder)

	log.Printf("changing to namespace %s", namespace)
	command := fmt.Sprintf("%s/entrypoint.sh", actionFolder)
	args := []string{namespace}
	cmd := exec.Command(command, args...)
	cmd.Env = os.Environ()
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", command, strings.Join(args, " "), string(o))
	}

	log.Printf("changing namespace output: \n%s\n", string(o))
	n := okteto.GetContext().Namespace
	if namespace != n {
		return fmt.Errorf("current namespace is %s, expected %s", n, namespace)
	}
	return nil
}

func executeDeleteNamespaceAction(namespace string) error {
	actionRepo := fmt.Sprintf("%s%s.git", githubHTTPSURL, deleteNamespacePath)
	actionFolder := strings.Split(deleteNamespacePath, "/")[1]
	log.Printf("cloning changing namespace repository: %s", actionRepo)
	if err := integration.CloneGitRepoWithBranch(actionRepo, oktetoVersion); err != nil {
		if err := integration.CloneGitRepo(actionRepo); err != nil {
			return err
		}
		log.Printf("cloned repo %s main branch\n", actionRepo)
	}
	log.Printf("cloned repo %s \n", actionRepo)
	defer integration.DeleteGitRepo(actionFolder)

	log.Printf("Deleting namespace %s", namespace)
	command := fmt.Sprintf("%s/entrypoint.sh", actionFolder)
	args := []string{namespace}
	cmd := exec.Command(command, args...)
	cmd.Env = os.Environ()
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", command, strings.Join(args, " "), string(o))
	}

	log.Printf("deleting namespace output: \n%s\n", string(o))
	return nil
}
