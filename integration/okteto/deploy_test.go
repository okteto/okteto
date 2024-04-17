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

package okteto

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/stretchr/testify/require"

	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
)

const (
	composeTemplate = `services:
  app:
    build: app
    command: echo -n $RABBITMQ_PASS > var.html && python -m http.server 8080
    ports:
    - 8080
  nginx:
    build: nginx
    volumes:
    - ./nginx/nginx.conf:/tmp/nginx.conf
    command: /bin/bash -c "envsubst < /tmp/nginx.conf > /etc/nginx/conf.d/default.conf && nginx -g 'daemon off;'"
    environment:
    - FLASK_SERVER_ADDR=app:8080
    ports:
    - 80:80
    depends_on:
      app:
        condition: service_started
    container_name: web-svc
    healthcheck:
      test: service nginx status || exit 1
      interval: 45s
      timeout: 5m
      retries: 5
      start_period: 30s`
	appDockerfile = "FROM python:alpine"
	nginxConf     = `server {
  listen 80;
  location / {
    proxy_pass http://$FLASK_SERVER_ADDR;
  }
}`
	composeFailTemplate = `services:
  app:
   image: alpine
   labels:
      1!"·$!31: ñÇ*^.,`
	oktetoCmdFailureTemplate = `deploy:
- name: Failed command
  command: exit 1
`
	oktetoCmdWithMaskValuesTemplate = `deploy:
- name: Mask command deploy
  command: echo $TOMASK
destroy:
  - name: Mask command destroy
    command: echo $TOMASK
`
	nginxDockerfile = `FROM nginx
COPY ./nginx.conf /tmp/nginx.conf
`
)

func TestDeploySuccessOutput(t *testing.T) {
	integration.SkipIfNotOktetoCluster(t)
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createComposeScenario(dir))

	testNamespace := integration.GetTestNamespace("SuccessOutput", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	require.NoError(t, commands.RunOktetoKubeconfig(oktetoPath, dir))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts)

	deployOptions := &commands.DeployOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	c, _, err := okteto.NewK8sClientProvider().Provide(kubeconfig.Get([]string{filepath.Join(dir, ".kube", "config")}))
	require.NoError(t, err)
	cmap, err := integration.GetConfigmap(context.Background(), testNamespace, fmt.Sprintf("okteto-git-%s", filepath.Base(dir)), c)
	require.NoError(t, err)

	uiOutput, err := base64.StdEncoding.DecodeString(cmap.Data["output"])
	require.NoError(t, err)

	var text oktetoLog.JSONLogFormat
	stageLines := map[string][]string{}
	prevStage := ""
	for _, l := range strings.Split(string(uiOutput), "\n") {
		if err := json.Unmarshal([]byte(l), &text); err != nil {
			if prevStage != "done" {
				t.Fatalf("not json format: %s", l)
			}
		}
		if _, ok := stageLines[text.Stage]; ok {
			stageLines[text.Stage] = append(stageLines[text.Stage], text.Message)
		} else {
			stageLines[text.Stage] = []string{text.Message}
		}
		prevStage = text.Stage
	}

	stagesToTest := []string{"Load manifest", "Building service app", "Deploying compose", "done"}
	for _, ss := range stagesToTest {
		if _, ok := stageLines[ss]; !ok {
			t.Fatalf("deploy didn't have the stage '%s'", ss)
		}
		if strings.HasPrefix(ss, "Building service") {
			if len(stageLines[ss]) < 5 {
				t.Fatalf("Not sending build output on stage %s. Output:%s", ss, stageLines[ss])
			}
		}

	}
}

func TestDeployWithNonSanitizedOK(t *testing.T) {
	integration.SkipIfNotOktetoCluster(t)
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createComposeScenario(dir))

	testNamespace := integration.GetTestNamespace("DeployWithNonSanitizedOK", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	require.NoError(t, commands.RunOktetoKubeconfig(oktetoPath, dir))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts)

	deployOptions := &commands.DeployOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
		Name:       "test/my deployment",
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	c, _, err := okteto.NewK8sClientProvider().Provide(kubeconfig.Get([]string{filepath.Join(dir, ".kube", "config")}))
	require.NoError(t, err)
	_, err = integration.GetConfigmap(context.Background(), testNamespace, fmt.Sprintf("okteto-git-%s", "test-my-deployment"), c)
	require.NoError(t, err)

}

func TestCmdFailOutput(t *testing.T) {
	integration.SkipIfNotOktetoCluster(t)
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createCommandFailureManifest(dir))

	testNamespace := integration.GetTestNamespace("CmdFailOutput", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	require.NoError(t, commands.RunOktetoKubeconfig(oktetoPath, dir))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts)

	deployOptions := &commands.DeployOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.Error(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	c, _, err := okteto.NewK8sClientProvider().Provide(kubeconfig.Get([]string{filepath.Join(dir, ".kube", "config")}))
	require.NoError(t, err)
	cmap, err := integration.GetConfigmap(context.Background(), testNamespace, fmt.Sprintf("okteto-git-%s", filepath.Base(dir)), c)
	require.NoError(t, err)

	uiOutput, err := base64.StdEncoding.DecodeString(cmap.Data["output"])
	require.NoError(t, err)

	var text oktetoLog.JSONLogFormat
	stageLines := map[string][]string{}
	prevStage := ""
	numErrors := 0
	for _, l := range strings.Split(string(uiOutput), "\n") {
		if err := json.Unmarshal([]byte(l), &text); err != nil {
			if prevStage != "done" {
				t.Fatalf("not json format: %s", l)
			}
		}
		if _, ok := stageLines[text.Stage]; ok {
			stageLines[text.Stage] = append(stageLines[text.Stage], text.Message)
		} else {
			stageLines[text.Stage] = []string{text.Message}
		}
		prevStage = text.Stage
		if text.Level == "error" {
			numErrors++
		}
	}

	require.Equal(t, 3, numErrors)
	stagesToTest := []string{"Load manifest", "Failed command", "done"}
	for _, ss := range stagesToTest {
		if _, ok := stageLines[ss]; !ok {
			t.Fatalf("deploy didn't have the stage '%s'", ss)
		}
	}
}

func TestRemoteMaskVariables(t *testing.T) {
	integration.SkipIfNotOktetoCluster(t)
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createCommandWitMaskValuesManifest(dir))

	testNamespace := integration.GetTestNamespace("RemoteMaskVariables", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	require.NoError(t, commands.RunOktetoKubeconfig(oktetoPath, dir))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts)

	deployOptions := &commands.DeployOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
		IsRemote:   true,
		Variables:  []string{"TOMASK=hola-mundo"},
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	c, _, err := okteto.NewK8sClientProvider().Provide(kubeconfig.Get([]string{filepath.Join(dir, ".kube", "config")}))
	require.NoError(t, err)
	cmap, err := integration.GetConfigmap(context.Background(), testNamespace, fmt.Sprintf("okteto-git-%s", filepath.Base(dir)), c)
	require.NoError(t, err)

	uiOutput, err := base64.StdEncoding.DecodeString(cmap.Data["output"])
	require.NoError(t, err)

	var text oktetoLog.JSONLogFormat
	stageLines := map[string][]string{}
	prevStage := ""
	numErrors := 0
	for _, l := range strings.Split(string(uiOutput), "\n") {
		if err := json.Unmarshal([]byte(l), &text); err != nil {
			if prevStage != "done" {
				t.Fatalf("not json format: %s", l)
			}
		}
		if _, ok := stageLines[text.Stage]; ok {
			stageLines[text.Stage] = append(stageLines[text.Stage], text.Message)
		} else {
			stageLines[text.Stage] = []string{text.Message}
		}
		prevStage = text.Stage
		if text.Level == "error" {
			numErrors++
		}
	}

	require.Equal(t, 0, numErrors)
	stagesToTest := []string{"Load manifest", "Mask command deploy", "done"}
	for _, ss := range stagesToTest {
		if _, ok := stageLines[ss]; !ok {
			t.Fatalf("deploy didn't have the stage '%s'", ss)
		}
		if ss == "Mask command deploy" {
			isMaskedValue := false
			for _, cmdLog := range stageLines[ss] {
				if cmdLog == "hola-mundo" {
					t.Fatal("deploy didn't mask the variable value.")
				}
				if cmdLog == "***" {
					isMaskedValue = true
				}
			}

			if !isMaskedValue {
				t.Fatal("deploy didn't mask the variable value.")
			}
		}
	}

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		IsRemote:   true,
	}

	o, err := commands.RunOktetoDestroyAndGetOutput(oktetoPath, destroyOptions)
	require.NoError(t, err)

	ologs := strings.Split(o, "\n")
	found := false
	for i, log := range ologs {
		if strings.HasSuffix(log, "Running stage 'Mask command destroy'") {
			if ologs[i+1] != "***" {
				t.Fatal("destroy didn't mask the variable value.")
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatal("destroy does not have the expected output.")
	}
}

func TestComposeFailOutput(t *testing.T) {
	integration.SkipIfNotOktetoCluster(t)
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createFailCompose(dir))

	testNamespace := integration.GetTestNamespace("ComposeFailOutput", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	require.NoError(t, commands.RunOktetoKubeconfig(oktetoPath, dir))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts)

	deployOptions := &commands.DeployOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.Error(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	c, _, err := okteto.NewK8sClientProvider().Provide(kubeconfig.Get([]string{filepath.Join(dir, ".kube", "config")}))
	require.NoError(t, err)
	cmap, err := integration.GetConfigmap(context.Background(), testNamespace, fmt.Sprintf("okteto-git-%s", filepath.Base(dir)), c)
	require.NoError(t, err)

	uiOutput, err := base64.StdEncoding.DecodeString(cmap.Data["output"])
	require.NoError(t, err)

	var text oktetoLog.JSONLogFormat
	stageLines := map[string][]string{}
	prevStage := ""
	numErrors := 0
	for _, l := range strings.Split(string(uiOutput), "\n") {
		if err := json.Unmarshal([]byte(l), &text); err != nil {
			if prevStage != "done" {
				t.Fatalf("not json format: %s", l)
			}
		}
		if _, ok := stageLines[text.Stage]; ok {
			stageLines[text.Stage] = append(stageLines[text.Stage], text.Message)
		} else {
			stageLines[text.Stage] = []string{text.Message}
		}
		prevStage = text.Stage
		if text.Level == "error" {
			numErrors++
		}
	}

	require.Equal(t, 3, numErrors)
	stagesToTest := []string{"Load manifest", "Deploying compose", "done"}
	for _, ss := range stagesToTest {
		if _, ok := stageLines[ss]; !ok {
			t.Fatalf("deploy didn't have the stage '%s'", ss)
		}
	}
}

func createCommandWitMaskValuesManifest(dir string) error {
	oktetoPath := filepath.Join(dir, "okteto.yml")
	oktetoContent := []byte(oktetoCmdWithMaskValuesTemplate)
	if err := os.WriteFile(oktetoPath, oktetoContent, 0600); err != nil {
		return err
	}
	return nil
}

func createCommandFailureManifest(dir string) error {
	oktetoPath := filepath.Join(dir, "okteto.yml")
	oktetoContent := []byte(oktetoCmdFailureTemplate)
	if err := os.WriteFile(oktetoPath, oktetoContent, 0600); err != nil {
		return err
	}
	return nil
}
func createFailCompose(dir string) error {
	composePath := filepath.Join(dir, "okteto-stack.yml")
	composeContent := []byte(composeFailTemplate)
	if err := os.WriteFile(composePath, composeContent, 0600); err != nil {
		return err
	}
	return nil
}

func createComposeScenario(dir string) error {
	if err := os.Mkdir(filepath.Join(dir, "nginx"), 0700); err != nil {
		return err
	}

	nginxPath := filepath.Join(dir, "nginx", "nginx.conf")
	nginxContent := []byte(nginxConf)
	if err := os.WriteFile(nginxPath, nginxContent, 0600); err != nil {
		return err
	}

	if err := createNginxDockerfile(dir); err != nil {
		return err
	}

	if err := createAppDockerfile(dir); err != nil {
		return err
	}

	composePath := filepath.Join(dir, "docker-compose.yml")
	composeContent := []byte(composeTemplate)
	if err := os.WriteFile(composePath, composeContent, 0600); err != nil {
		return err
	}

	return nil
}

func createAppDockerfile(dir string) error {
	if err := os.Mkdir(filepath.Join(dir, "app"), 0700); err != nil {
		return err
	}

	appDockerfilePath := filepath.Join(dir, "app", "Dockerfile")
	appDockerfileContent := []byte(appDockerfile)
	if err := os.WriteFile(appDockerfilePath, appDockerfileContent, 0600); err != nil {
		return err
	}
	return nil
}

func createNginxDockerfile(dir string) error {
	nginxDockerfilePath := filepath.Join(dir, "nginx", "Dockerfile")
	nginxDockerfileContent := []byte(nginxDockerfile)
	if err := os.WriteFile(nginxDockerfilePath, nginxDockerfileContent, 0600); err != nil {
		return err
	}
	return nil
}
