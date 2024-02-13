//go:build integration
// +build integration

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

package push

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/require"
)

var (
	user          = ""
	token         = ""
	kubectlBinary = "kubectl"
	appsSubdomain = "cloud.okteto.net"
)

const (
	timeout    = 300 * time.Second
	dockerfile = `FROM python:alpine
WORKDIR /usr/src/app
COPY index.html index.html
ENTRYPOINT ["python", "-m", "http.server", "8080"]`
	oktetoManifest = `name: push-test
autocreate: true
image:
  name: okteto.dev/push-test:dev
  context: .
forward:
  - 8080:8080
sync:
  - .:/usr/src/app
persistentVolume:
  enabled: false
`
)

func TestMain(m *testing.M) {
	if u, ok := os.LookupEnv(model.OktetoUserEnvVar); !ok {
		log.Println("OKTETO_USER is not defined")
		os.Exit(1)
	} else {
		user = u
	}

	if v := os.Getenv(model.OktetoAppsSubdomainEnvVar); v != "" {
		appsSubdomain = v
	}

	if runtime.GOOS == "windows" {
		kubectlBinary = "kubectl.exe"
	}
	token = integration.GetToken()
	if _, err := exec.LookPath(kubectlBinary); err != nil {
		log.Printf("kubectl is not in the path: %s", err)
		os.Exit(1)
	}

	exitCode := m.Run()
	os.Exit(exitCode)
}

func TestPush(t *testing.T) {
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)
	dir := t.TempDir()

	testNamespace := integration.GetTestNamespace("Push", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts)
	require.NoError(t, commands.RunOktetoKubeconfig(oktetoPath, dir))
	c, _, err := okteto.NewK8sClientProvider().Provide(kubeconfig.Get([]string{filepath.Join(dir, ".kube", "config")}))
	require.NoError(t, err)

	require.NoError(t, createDockerfile(dir))
	require.NoError(t, createOktetoManifest(dir))
	require.NoError(t, createIndexHTML(dir))

	require.NoError(t, commands.RunOktetoPush(oktetoPath, dir))

	endpoint := fmt.Sprintf("https://push-test-%s.%s/index.html", testNamespace, appsSubdomain)
	require.NoError(t, waitUntilUpdatedContent(endpoint, dockerfile, timeout))

	d, err := integration.GetDeployment(context.Background(), testNamespace, "push-test", c)
	require.NoError(t, err)

	imageName := fmt.Sprintf("registry.%s/%s/push-test:okteto", appsSubdomain, testNamespace)
	require.Equal(t, imageName, d.Spec.Template.Spec.Containers[0].Image)

}

func createDockerfile(dir string) error {
	appDockerfilePath := filepath.Join(dir, "Dockerfile")
	appDockerfileContent := []byte(dockerfile)
	if err := os.WriteFile(appDockerfilePath, appDockerfileContent, 0600); err != nil {
		return err
	}
	return nil
}

func createIndexHTML(dir string) error {
	appDockerfilePath := filepath.Join(dir, "index.html")
	appDockerfileContent := []byte(dockerfile)
	if err := os.WriteFile(appDockerfilePath, appDockerfileContent, 0600); err != nil {
		return err
	}
	return nil
}

func createOktetoManifest(dir string) error {
	appDockerfilePath := filepath.Join(dir, "okteto.yml")
	appDockerfileContent := []byte(oktetoManifest)
	if err := os.WriteFile(appDockerfilePath, appDockerfileContent, 0600); err != nil {
		return err
	}
	return nil
}

func waitUntilUpdatedContent(url, expectedContent string, timeout time.Duration) error {
	ticker := time.NewTicker(1 * time.Second)
	to := time.NewTicker(timeout)
	contentTimeout := 5 * time.Second
	retry := 0
	for {
		select {
		case <-to.C:
			return fmt.Errorf("%s without updating %s to %s", timeout.String(), url, expectedContent)
		case <-ticker.C:
			retry++
			content := integration.GetContentFromURL(url, contentTimeout)
			if content == "" {
				continue
			}
			if content != expectedContent {
				if retry%10 == 0 {
					log.Printf("expected updated content to be %s, got %s\n", expectedContent, content)
				}
				continue
			}
			return nil
		}
	}
}
