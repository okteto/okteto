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
	"github.com/okteto/okteto/integration/commands"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
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

type deployment struct {
	Name string
}

func TestApplyPipeline(t *testing.T) {
	integration.SkipIfWindows(t)

	namespace := integration.GetTestNamespace("applyaction", user)
	oktetoPath, err := integration.GetOktetoPath()
	assert.NoError(t, err)

	assert.NoError(t, executeCreateNamespaceAction(namespace))
	assert.NoError(t, commands.RunOktetoKubeconfig(oktetoPath, ""))
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
	if err := writeDeployment(actionManifestTemplate, namespace, dPath); err != nil {
		return err
	}

	actionRepo := fmt.Sprintf("%s%s.git", githubHTTPSURL, applyPath)
	actionFolder := strings.Split(applyPath, "/")[1]
	log.Printf("cloning apply repository: %s", actionRepo)
	if err := integration.CloneGitRepoWithBranch(actionRepo, oktetoVersion); err != nil {
		if err := integration.CloneGitRepo(actionRepo); err != nil {
			return err
		}
		log.Printf("cloned repo %s main branch\n", actionRepo)
	}
	log.Printf("cloned repo %s \n", actionRepo)
	defer integration.DeleteGitRepo(actionFolder)

	command := fmt.Sprintf("%s/entrypoint.sh", actionFolder)
	args := []string{dPath, namespace}

	cmd := exec.Command(command, args...)
	kubepath := config.GetKubeconfigPath()[0]
	log.Printf("Using kubeconfig: '%s'", kubepath)
	if _, err := os.Stat(kubepath); err != nil {
		log.Printf("could not get kubepath: %s", err)
	}
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", kubepath))
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", command, strings.Join(args, " "), string(o))
	}
	c, _, err := okteto.NewK8sClientProvider().Provide(kubeconfig.Get([]string{kubepath}))
	if err != nil {
		return err
	}

	d, err := integration.GetDeployment(context.Background(), namespace, namespace, c)
	if err != nil || d == nil {
		return fmt.Errorf("Could not get deployment %s", namespace)
	}
	return nil
}

func writeDeployment(template *template.Template, name, path string) error {
	dFile, err := os.Create(path)
	if err != nil {
		return err
	}

	if err := template.Execute(dFile, deployment{Name: name}); err != nil {
		return err
	}
	defer func() {
		if err := dFile.Close(); err != nil {
			oktetoLog.Debugf("Error closing file %s: %s", path, err)
		}
	}()

	return nil
}
