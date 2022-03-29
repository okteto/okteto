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

package integration

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
)

const (
	githubHttpsUrl      = "https://github.com/"
	applyPath           = "okteto/apply"
	buildPath           = "okteto/build"
	createNamespacePath = "okteto/create-namespace"
	deleteNamespacePath = "okteto/delete-namespace"
	deployStackPath     = "okteto/deploy-stack"
	destroyPipelinePath = "okteto/destroy-pipeline"
	deployPreviewPath   = "okteto/deploy-preview"
	destroyPreviewPath  = "okteto/destroy-preview"
	destroyStackPath    = "okteto/destroy-stack"
	loginPath           = "okteto/login"
	contextPath         = "okteto/context"
	namespacePath       = "okteto/namespace"
	pipelinePath        = "okteto/pipeline"
	pushPath            = "okteto/push"

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
	stackFile = `
name: test
services:
  app:
    image: nginx
    ports:
      - 8080:80
`

	githubUrl = "https://github.com"
)

var (
	actionManifestTemplate = template.Must(template.New("deployment").Parse(deploymentManifestFormat))
	pipelineRepo           = "okteto/movies"
	pipelineRepoURL        = "git@github.com:okteto/movies.git"
	pipelineFolder         = "movies"
)

func TestApplyPipeline(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("this test is not required for windows e2e tests")
		return
	}

	ctx := context.Background()
	namespace := getTestNamespace()
	oktetoPath, err := getOktetoPath(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := executeCreateNamespaceAction(ctx, namespace); err != nil {
		t.Fatalf("Create namespace action failed: %s", err.Error())
	}

	if err := updateKubeConfig(oktetoPath); err != nil {
		t.Fatalf("Could not update kubeconfig: %s", err.Error())
	}

	if err := executeApply(ctx, namespace); err != nil {
		t.Fatalf("Apply action failed: %s", err.Error())
	}

	if err := executeDeleteNamespaceAction(ctx, namespace); err != nil {
		t.Fatalf("Delete namespace action failed: %s", err.Error())
	}
}

func TestBuildActionPipeline(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("this test is not required for windows e2e tests")
		return
	}

	ctx := context.Background()
	namespace := getTestNamespace()

	if err := executeCreateNamespaceAction(ctx, namespace); err != nil {
		t.Fatalf("Create namespace action failed: %s", err.Error())
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tempDir, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	log.Printf("created tempdir: %s", tempDir)
	dockerfilePath := filepath.Join(tempDir, "Dockerfile")
	dockerfileContent := []byte("FROM alpine")
	if err := os.WriteFile(dockerfilePath, dockerfileContent, 0644); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(tempDir, "okteto.yml")
	manifestContent := []byte(`
build:
  alpine:
    context: .
`)
	if err := os.WriteFile(manifestPath, manifestContent, 0644); err != nil {
		t.Fatal(err)
	}

	actionRepo := fmt.Sprintf("%s%s.git", githubHttpsUrl, buildPath)
	actionFolder := path.Join(cwd, strings.Split(buildPath, "/")[1])
	log.Printf("cloning build action repository: %s", actionRepo)
	if err := cloneGitRepo(ctx, actionRepo); err != nil {
		t.Fatal(err)
	}
	log.Printf("cloned repo %s \n", actionRepo)
	defer deleteGitRepo(ctx, actionFolder)

	log.Printf("building image")
	command := fmt.Sprintf("%s/entrypoint.sh", actionFolder)

	// test v1 build with dockerfile
	args := []string{fmt.Sprintf("okteto.dev/%s:latest", namespace), dockerfilePath, tempDir}

	cmd := exec.Command(command, args...)
	cmd.Env = os.Environ()
	cmd.Dir = tempDir
	o, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %s", command, strings.Join(args, " "), string(o))
	}

	// test v2 build with manifest
	argsV2 := []string{"", manifestPath}
	cmdV2 := exec.Command(command, argsV2...)
	cmdV2.Env = os.Environ()
	cmdV2.Dir = tempDir
	o2, err := cmdV2.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %s", command, strings.Join(args, " "), string(o2))
	}

}

func TestNamespaceActionsPipeline(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("this test is not required for windows e2e tests")
		return
	}

	ctx := context.Background()
	namespace := getTestNamespace()

	if err := executeCreateNamespaceAction(ctx, namespace); err != nil {
		t.Fatalf("Create namespace action failed: %s", err.Error())
	}
	if err := executeChangeNamespaceAction(ctx, namespace); err != nil {
		t.Fatalf("Change namespace action failed: %s", err.Error())
	}
	if err := executeDeleteNamespaceAction(ctx, namespace); err != nil {
		t.Fatalf("Delete namespace action failed: %s", err.Error())
	}
}

func TestLoginActionPipeline(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("this test is not required for windows e2e tests")
		return
	}

	ctx := context.Background()
	var remove bool
	if _, err := os.Stat(config.GetOktetoContextFolder()); err != nil {
		remove = true
	}
	if err := executeLoginAction(ctx); err != nil {
		t.Fatalf("Login action failed: %s", err.Error())
	}

	if remove {
		if err := os.RemoveAll(config.GetOktetoContextFolder()); err != nil {
			t.Fatalf("Removing failed: %s", err.Error())
		}
	}

}

func TestContextAction(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("this test is not required for windows e2e tests")
		return
	}
	if os.Getenv(model.OktetoSkipContextTestEnvVar) != "" {
		t.Skip("this test is not required because of 'OKTETO_SKIP_CONTEXT_TEST' env var")
		return
	}

	ctx := context.Background()
	var remove bool
	if _, err := os.Stat(config.GetOktetoContextFolder()); err != nil {
		remove = true
	}
	if err := executeContextAction(ctx); err != nil {
		t.Fatalf("Context action failed: %s", err.Error())
	}

	if remove {
		if err := os.RemoveAll(config.GetOktetoContextFolder()); err != nil {
			t.Fatalf("Removing failed: %s", err.Error())
		}
	}

}

func TestPipelineActions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("this test is not required for windows e2e tests")
		return
	}

	ctx := context.Background()
	namespace := getTestNamespace()

	if err := executeCreateNamespaceAction(ctx, namespace); err != nil {
		t.Fatalf("Create namespace action failed: %s", err.Error())
	}

	if err := executeDeployPipelineAction(ctx, namespace); err != nil {
		t.Fatalf("Deploy pipeline action failed: %s", err.Error())
	}
	if err := executeDestroyPipelineAction(ctx, namespace); err != nil {
		t.Fatalf("destroy pipeline action failed: %s", err.Error())
	}

	if err := executeDeleteNamespaceAction(ctx, namespace); err != nil {
		t.Fatalf("Delete namespace action failed: %s", err.Error())
	}
}

func TestPreviewActions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("this test is not required for windows e2e tests")
		return
	}

	ctx := context.Background()
	namespace := getTestNamespace()

	if err := executeDeployPreviewAction(ctx, namespace); err != nil {
		t.Fatalf("Deploy preview action failed: %s", err.Error())
	}

	if err := executeDestroyPreviewAction(ctx, namespace); err != nil {
		t.Fatalf("Destroy preview action failed: %s", err.Error())
	}
}

func TestPushAction(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("this test is not required for windows e2e tests")
		return
	}

	ctx := context.Background()
	namespace := getTestNamespace()
	oktetoPath, err := getOktetoPath(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := executeCreateNamespaceAction(ctx, namespace); err != nil {
		t.Fatalf("Create namespace action failed: %s", err.Error())
	}

	if err := updateKubeConfig(oktetoPath); err != nil {
		t.Fatalf("Could not update kubeconfig: %s", err.Error())
	}

	if err := executeApply(ctx, namespace); err != nil {
		t.Fatalf("Apply action failed: %s", err.Error())
	}

	if err := executePushAction(ctx, namespace); err != nil {
		t.Fatalf("Push action failed: %s", err.Error())
	}

	if err := executeDeleteNamespaceAction(ctx, namespace); err != nil {
		t.Fatalf("Delete namespace action failed: %s", err.Error())
	}
}

func TestStacksActions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("this test is not required for windows e2e tests")
		return
	}

	ctx := context.Background()
	namespace := getTestNamespace()

	if err := executeCreateNamespaceAction(ctx, namespace); err != nil {
		t.Fatalf("Create namespace action failed: %s", err.Error())
	}

	dir, err := os.MkdirTemp("", namespace)
	if err != nil {
		t.Fatal(err)
	}
	log.Printf("created tempdir: %s", dir)
	filePath := filepath.Join(dir, "okteto-stack.yaml")
	if err := os.WriteFile(filePath, []byte(stackFile), 0644); err != nil {
		t.Fatal(err)
	}

	if err := executeDeployStackAction(ctx, namespace, filePath); err != nil {
		t.Fatalf("Deploy stack action failed: %s", err.Error())
	}
	if err := executeDestroyStackAction(ctx, namespace, filePath); err != nil {
		t.Fatalf("Destroy stack action failed: %s", err.Error())
	}

	if err := executeDeleteNamespaceAction(ctx, namespace); err != nil {
		t.Fatalf("Delete namespace action failed: %s", err.Error())
	}
}

func getTestNamespace() string {
	tName := fmt.Sprintf("TestAction-%s", runtime.GOOS)
	name := strings.ToLower(fmt.Sprintf("%s-%d", tName, time.Now().Unix()))
	namespace := fmt.Sprintf("%s-%s", name, user)
	return namespace
}

func executeCreateNamespaceAction(ctx context.Context, namespace string) error {
	okteto.CurrentStore = nil
	actionRepo := fmt.Sprintf("%s%s.git", githubHttpsUrl, createNamespacePath)
	actionFolder := strings.Split(createNamespacePath, "/")[1]
	log.Printf("cloning create namespace repository: %s", actionRepo)
	if err := cloneGitRepo(ctx, actionRepo); err != nil {
		return err
	}
	log.Printf("cloned repo %s \n", actionRepo)
	defer deleteGitRepo(ctx, actionFolder)

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
	n := okteto.Context().Namespace
	if namespace != n {
		return fmt.Errorf("current namespace is %s, expected %s", n, namespace)
	}

	return nil
}

func executeChangeNamespaceAction(ctx context.Context, namespace string) error {
	okteto.CurrentStore = nil
	actionRepo := fmt.Sprintf("%s%s.git", githubHttpsUrl, namespacePath)
	actionFolder := strings.Split(namespacePath, "/")[1]
	log.Printf("cloning changing namespace repository: %s", actionRepo)
	if err := cloneGitRepo(ctx, actionRepo); err != nil {
		return err
	}
	log.Printf("cloned repo %s \n", actionRepo)
	defer deleteGitRepo(ctx, actionFolder)

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
	n := okteto.Context().Namespace
	if namespace != n {
		return fmt.Errorf("current namespace is %s, expected %s", n, namespace)
	}
	return nil
}

func executeDeleteNamespaceAction(ctx context.Context, namespace string) error {
	actionRepo := fmt.Sprintf("%s%s.git", githubHttpsUrl, deleteNamespacePath)
	actionFolder := strings.Split(deleteNamespacePath, "/")[1]
	log.Printf("cloning changing namespace repository: %s", actionRepo)
	if err := cloneGitRepo(ctx, actionRepo); err != nil {
		return err
	}
	log.Printf("cloned repo %s \n", actionRepo)
	defer deleteGitRepo(ctx, actionFolder)

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

func executeDeployPipelineAction(ctx context.Context, namespace string) error {
	actionRepo := fmt.Sprintf("%s%s.git", githubHttpsUrl, pipelinePath)
	actionFolder := strings.Split(pipelinePath, "/")[1]
	log.Printf("cloning pipeline repository: %s", actionRepo)
	err := cloneGitRepo(ctx, actionRepo)
	if err != nil {
		return err
	}
	log.Printf("cloned repo %s \n", actionRepo)
	defer deleteGitRepo(ctx, actionFolder)
	os.Setenv(model.GithubRepositoryEnvVar, pipelineRepo)
	os.Setenv(model.GithubRefEnvVar, "master")
	os.Setenv(model.GithubServerURLEnvVar, githubUrl)

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
	pipeline, err := oktetoClient.GetPipelineByName(ctx, "movies")
	if err != nil || pipeline == nil {
		return fmt.Errorf("Could not get deployment %s", namespace)
	}
	return nil
}

func executeDestroyPipelineAction(ctx context.Context, namespace string) error {
	actionRepo := fmt.Sprintf("%s%s.git", githubHttpsUrl, destroyPipelinePath)
	actionFolder := strings.Split(destroyPipelinePath, "/")[1]
	log.Printf("cloning destroy pipeline repository: %s", actionRepo)
	if err := cloneGitRepo(ctx, actionRepo); err != nil {
		return err
	}
	log.Printf("cloned repo %s \n", actionRepo)
	defer deleteGitRepo(ctx, actionFolder)

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

func executeApply(ctx context.Context, namespace string) error {

	dir, err := os.MkdirTemp("", namespace)
	if err != nil {
		return err
	}
	log.Printf("created tempdir: %s", dir)
	dPath := filepath.Join(dir, "deployment.yaml")
	if err := writeDeployment(actionManifestTemplate, namespace, dPath); err != nil {
		return err
	}

	actionRepo := fmt.Sprintf("%s%s.git", githubHttpsUrl, applyPath)
	actionFolder := strings.Split(applyPath, "/")[1]
	log.Printf("cloning apply repository: %s", actionRepo)
	err = cloneGitRepo(ctx, actionRepo)
	if err != nil {
		return err
	}
	log.Printf("cloned repo %s \n", actionRepo)
	defer deleteGitRepo(ctx, actionFolder)

	log.Printf("creating namespace %s", namespace)
	command := fmt.Sprintf("%s/entrypoint.sh", actionFolder)
	args := []string{dPath, namespace}

	cmd := exec.Command(command, args...)
	cmd.Env = os.Environ()
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", command, strings.Join(args, " "), string(o))
	}

	d, err := getDeployment(ctx, namespace, namespace)
	if err != nil || d == nil {
		return fmt.Errorf("Could not get deployment %s", namespace)
	}
	return nil
}

func executePushAction(ctx context.Context, namespace string) error {
	dir, err := os.MkdirTemp("", namespace)
	if err != nil {
		return err
	}
	log.Printf("created tempdir: %s", dir)
	dockerfilePath := filepath.Join(dir, "Dockerfile")
	dockerfileContent := []byte("FROM alpine")
	if err := os.WriteFile(dockerfilePath, dockerfileContent, 0644); err != nil {
		return err
	}

	actionRepo := fmt.Sprintf("%s%s.git", githubHttpsUrl, pushPath)
	actionFolder := strings.Split(pushPath, "/")[1]
	log.Printf("cloning push repository: %s", actionRepo)
	err = cloneGitRepo(ctx, actionRepo)
	if err != nil {
		return err
	}
	log.Printf("cloned repo %s \n", actionRepo)
	defer deleteGitRepo(ctx, actionFolder)

	log.Printf("pushing %s", namespace)
	command := fmt.Sprintf("%s/entrypoint.sh", actionFolder)
	args := []string{namespace, namespace, "", dir}

	cmd := exec.Command(command, args...)
	cmd.Env = os.Environ()
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", command, strings.Join(args, " "), string(o))
	}

	d, err := getDeployment(ctx, namespace, namespace)
	if err != nil || d == nil {
		return fmt.Errorf("Could not get deployment %s", namespace)
	}
	if d.Spec.Template.Spec.Containers[0].Image == "alpine" {
		return fmt.Errorf("Not updated image")
	}
	return nil
}

func executeDeployStackAction(ctx context.Context, namespace, filePath string) error {

	actionRepo := fmt.Sprintf("%s%s.git", githubHttpsUrl, deployStackPath)
	actionFolder := strings.Split(deployStackPath, "/")[1]
	log.Printf("cloning pipeline repository: %s", actionRepo)
	err := cloneGitRepo(ctx, actionRepo)
	if err != nil {
		return err
	}
	log.Printf("cloned repo %s \n", actionRepo)
	defer deleteGitRepo(ctx, actionFolder)

	log.Printf("creating namespace %s", namespace)
	command := fmt.Sprintf("%s/entrypoint.sh", actionFolder)
	args := []string{namespace, "", "", filePath}

	cmd := exec.Command(command, args...)
	cmd.Env = os.Environ()
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", command, strings.Join(args, " "), string(o))
	}

	return nil
}

func executeDestroyStackAction(ctx context.Context, namespace, filePath string) error {
	actionRepo := fmt.Sprintf("%s%s.git", githubHttpsUrl, destroyStackPath)
	actionFolder := strings.Split(destroyStackPath, "/")[1]
	log.Printf("cloning destroy path repository: %s", actionRepo)
	if err := cloneGitRepo(ctx, actionRepo); err != nil {
		return err
	}
	log.Printf("cloned repo %s \n", actionRepo)
	defer deleteGitRepo(ctx, actionFolder)

	log.Printf("Deleting compose %s", namespace)
	command := fmt.Sprintf("%s/entrypoint.sh", actionFolder)
	args := []string{namespace, "", filePath}
	cmd := exec.Command(command, args...)
	cmd.Env = os.Environ()
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", command, strings.Join(args, " "), string(o))
	}

	log.Printf("destroy compose output: \n%s\n", string(o))
	return nil
}

func executeLoginAction(ctx context.Context) error {
	token := os.Getenv(model.OktetoTokenEnvVar)
	if token == "" {
		token = okteto.Context().Token
	}

	actionRepo := fmt.Sprintf("%s%s.git", githubHttpsUrl, loginPath)
	actionFolder := strings.Split(loginPath, "/")[1]
	log.Printf("cloning build action repository: %s", actionRepo)
	if err := cloneGitRepo(ctx, actionRepo); err != nil {
		return err
	}
	log.Printf("cloned repo %s \n", actionRepo)
	defer deleteGitRepo(ctx, actionFolder)

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
	log.Printf("logging output: \n%s\n", string(o))
	return nil
}

func executeContextAction(ctx context.Context) error {
	token := os.Getenv(model.OktetoTokenEnvVar)
	if token == "" {
		token = okteto.Context().Token
	}

	actionRepo := fmt.Sprintf("%s%s.git", githubHttpsUrl, contextPath)
	actionFolder := strings.Split(contextPath, "/")[1]
	log.Printf("cloning build action repository: %s", actionRepo)
	if err := cloneGitRepo(ctx, actionRepo); err != nil {
		return err
	}
	log.Printf("cloned repo %s \n", actionRepo)
	defer deleteGitRepo(ctx, actionFolder)

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

func executeDeployPreviewAction(ctx context.Context, namespace string) error {
	oktetoPath, err := getOktetoPath(ctx)
	if err != nil {
		return err
	}
	actionRepo := fmt.Sprintf("%s%s.git", githubHttpsUrl, deployPreviewPath)
	actionFolder := strings.Split(deployPreviewPath, "/")[1]
	log.Printf("cloning destroy path repository: %s", actionRepo)
	if err := cloneGitRepo(ctx, actionRepo); err != nil {
		return err
	}
	log.Printf("cloned repo %s \n", actionRepo)
	defer deleteGitRepo(ctx, actionFolder)

	log.Printf("Deploying preview %s", namespace)
	command := oktetoPath
	args := []string{"preview", "deploy", namespace, "--scope", "personal", "--branch", "master", "--repository", fmt.Sprintf("%s/%s", githubUrl, pipelineRepo), "--wait"}
	cmd := exec.Command(command, args...)
	cmd.Env = os.Environ()
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", command, strings.Join(args, " "), string(o))
	}

	log.Printf("destroy preview output: \n%s\n", string(o))
	return nil
}

func executeDestroyPreviewAction(ctx context.Context, namespace string) error {
	actionRepo := fmt.Sprintf("%s%s.git", githubHttpsUrl, destroyPreviewPath)
	actionFolder := strings.Split(destroyPreviewPath, "/")[1]
	log.Printf("cloning destroy path repository: %s", actionRepo)
	if err := cloneGitRepo(ctx, actionRepo); err != nil {
		return err
	}
	log.Printf("cloned repo %s \n", actionRepo)
	defer deleteGitRepo(ctx, actionFolder)

	log.Printf("Deleting preview %s", namespace)
	command := "chmod"
	entrypointPath := fmt.Sprintf("%s/entrypoint.sh", actionFolder)
	args := []string{"+x", entrypointPath}
	cmd := exec.Command(command, args...)
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", command, strings.Join(args, " "), string(o))
	}

	args = []string{namespace}
	cmd = exec.Command(entrypointPath, args...)
	cmd.Env = os.Environ()
	o, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", entrypointPath, strings.Join(args, " "), string(o))
	}

	log.Printf("destroy preview output: \n%s\n", string(o))
	return nil
}
