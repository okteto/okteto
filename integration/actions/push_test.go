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

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/require"
)

const pushPath = "okteto/push"

func TestPushAction(t *testing.T) {
	integration.SkipIfWindows(t)

	namespace := integration.GetTestNamespace("pushactions", user)
	oktetoPath, err := integration.GetOktetoPath()
	if err != nil {
		t.Fatal(err)
	}

	require.NoError(t, executeCreateNamespaceAction(namespace))
	require.NoError(t, commands.RunOktetoKubeconfig(oktetoPath, ""))
	require.NoError(t, executeApply(namespace))
	require.NoError(t, executePushAction(t, namespace))
	require.NoError(t, executeDeleteNamespaceAction(namespace))
}

func executePushAction(t *testing.T, namespace string) error {
	dockerfile, err := createDockerfile(t)
	if err != nil {
		return err
	}
	actionRepo := fmt.Sprintf("%s%s.git", githubHTTPSURL, pushPath)
	actionFolder := strings.Split(pushPath, "/")[1]
	log.Printf("cloning push repository: %s", actionRepo)
	if err := integration.CloneGitRepoWithBranch(actionRepo, oktetoVersion); err != nil {
		if err := integration.CloneGitRepo(actionRepo); err != nil {
			return err
		}
		log.Printf("cloned repo %s main branch\n", actionRepo)
	}
	log.Printf("cloned repo %s \n", actionRepo)
	defer integration.DeleteGitRepo(actionFolder)

	log.Printf("pushing %s", namespace)
	command := fmt.Sprintf("%s/entrypoint.sh", actionFolder)
	args := []string{namespace, namespace, "", filepath.Dir(dockerfile)}

	cmd := exec.Command(command, args...)
	kubepath := config.GetKubeconfigPath()[0]
	log.Printf("Using kubeconfig: '%s'", kubepath)
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
	if d.Spec.Template.Spec.Containers[0].Image == "alpine" {
		return fmt.Errorf("Not updated image")
	}
	return nil
}
