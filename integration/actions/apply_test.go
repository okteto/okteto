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
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/okteto/okteto/integration"
	"github.com/stretchr/testify/assert"
)

const (
	applyPath = "okteto/apply"

	deploymentManifestFormat = `
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: {{ .Name }}
    spec:
      replicas: 1
      selector:
        matchLabels:
          app: {{ .Name }}
      template:
        metadata:
          labels:
            app: {{ .Name }}
        spec:
          containers:
          - name: test
            image: alpine
            command: [ "sh", "-c", "--" ]
            args: [ "while true; do sleep 30; done;" ]
`
)

var (
	actionManifestTemplate = template.Must(template.New("deployment").Parse(deploymentManifestFormat))
)

func TestApplyPipeline(t *testing.T) {
	integration.SkipIfWindows(t)

	namespace := getTestNamespace()
	oktetoPath, err := integration.GetOktetoPath()
	assert.NoError(t, err)

	assert.NoError(t, executeCreateNamespaceAction(namespace))
	assert.NoError(t, integration.RunOktetoKubeconfig(oktetoPath))
	assert.NoError(t, executeApply(namespace))
	assert.NoError(t, executeDeleteNamespaceAction(namespace))
}

func executeApply(namespace string) error {

	dir, err := os.MkdirTemp("", namespace)
	if err != nil {
		return err
	}
	log.Printf("created tempdir: %s", dir)
	defer os.RemoveAll(dir)

	dPath := filepath.Join(dir, "deployment.yaml")
	if err := integration.WriteDeployment(actionManifestTemplate, namespace, dPath); err != nil {
		return err
	}

	actionRepo := fmt.Sprintf("%s%s.git", githubHTTPSURL, applyPath)
	actionFolder := strings.Split(applyPath, "/")[1]
	log.Printf("cloning apply repository: %s", actionRepo)
	err = integration.CloneGitRepo(actionRepo)
	if err != nil {
		return err
	}
	log.Printf("cloned repo %s \n", actionRepo)
	defer integration.DeleteGitRepo(actionFolder)

	log.Printf("creating namespace %s", namespace)
	command := fmt.Sprintf("%s/entrypoint.sh", actionFolder)
	args := []string{dPath, namespace}

	cmd := exec.Command(command, args...)
	cmd.Env = os.Environ()
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", command, strings.Join(args, " "), string(o))
	}

	d, err := integration.GetDeployment(context.Background(), namespace, namespace)
	if err != nil || d == nil {
		return fmt.Errorf("Could not get deployment %s", namespace)
	}
	return nil
}
