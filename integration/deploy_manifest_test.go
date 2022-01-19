//go:build integration
// +build integration

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
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
)

// okteto deploy - build the images
// okteto deploy  - does not build the images
// okteto deploy --build - rebuilds the images

func TestDeployFromManifest(t *testing.T) {
	ctx := context.Background()
	oktetoPath, err := getOktetoPath(ctx)
	if err != nil {
		t.Fatal(err)
	}
	testID := strings.ToLower(fmt.Sprintf("DeployFromManifest-%s-%d", runtime.GOOS, time.Now().Unix()))
	testNamespace := fmt.Sprintf("%s-%s", testID, user)

	const (
		gitRepo          = "git@github.com:okteto/go-getting-started.git"
		repoDir          = "go-getting-started"
		manifestFilename = "okteto.yml"
		chartDir         = "chart"
		chartFilename    = "Chart.yaml"
		templateFilename = "k8s.yaml"
		releaseName      = "hello-world"
	)

	expectedImage := fmt.Sprintf("%s/%s/%s-app:okteto", okteto.Context().Registry, testNamespace, repoDir)
	t.Logf("expected image tag to be built and deployed: %s", expectedImage)

	t.Run(testID, func(t *testing.T) {

		startNamespace := getCurrentNamespace()
		defer changeToNamespace(ctx, oktetoPath, startNamespace)

		if err := createNamespace(ctx, oktetoPath, testNamespace); err != nil {
			t.Fatal(err)
		}
		defer deleteNamespace(ctx, oktetoPath, testNamespace)

		if err := cloneGitRepo(ctx, gitRepo); err != nil {
			t.Fatal(err)
		}
		defer deleteGitRepo(ctx, repoDir)

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}

		manifestPath := filepath.Join(cwd, repoDir)
		t.Logf("manifestPath: %s", manifestPath)

		manifestContent := `build:
  app:
    context: .
deploy:
  - helm upgrade --install hello-world chart --set app.image=${build.app.image}`

		if err := writeFile(manifestPath, manifestFilename, manifestContent); err != nil {
			t.Fatal(err)
		}

		chartContent := `apiVersion: v2
name: hello-world
description: A React application in Kubernetes
type: application
version: 0.1.0
appVersion: 1.0.0`

		templateContent := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: hello-world
spec:
  replicas: 1
  selector:
    matchLabels:
      app: hello-world
  template:
    metadata:
      labels:
        app: hello-world
    spec:
      containers:
        - image: {{ .Values.app.image }}
          name: hello-world

---
apiVersion: v1
kind: Service
metadata:
  name: hello-world
  annotations:
    dev.okteto.com/auto-ingress: "true"
spec:
  type: ClusterIP
  ports:
    - name: "hello-world"
      port: 8080
  selector:
    app: hello-world`

		pathToChartDir := filepath.Join(cwd, repoDir, chartDir)
		if err := os.Mkdir(pathToChartDir, 0777); err != nil {
			t.Fatal(err)
		}
		if err := writeFile(pathToChartDir, chartFilename, chartContent); err != nil {
			t.Fatal(err)
		}

		pathToTemplateDir := filepath.Join(pathToChartDir, "templates")
		if err := os.Mkdir(pathToTemplateDir, 0777); err != nil {
			t.Fatal(err)
		}
		if err := writeFile(pathToTemplateDir, templateFilename, templateContent); err != nil {
			t.Fatal(err)
		}

		cmd := exec.Command(oktetoPath, "deploy", "-l", "debug")
		cmd.Env = append(os.Environ(), "OKTETO_ENABLE_MANIFEST_V2=true")
		cmd.Dir = repoDir
		o, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("okteto deploy failed: %s - %s", string(o), err)
		}
		output := string(o)

		imageWithDigest, err := registry.GetImageTagWithDigest(expectedImage)
		if err != nil {
			t.Fatalf("image is not at registry: %s", err.Error())
		}
		sha := strings.SplitN(imageWithDigest, "@", 2)[1]

		if ok := expectBuiltImageNotFound(output); !ok {
			t.Fatalf("expected to build image before deploy")
		}

		if ok := expectHelmInstallation(output, releaseName, testNamespace); !ok {
			t.Fatalf("expected helm first installation")
		}

		if ok := expectEnvSetting(output, testNamespace, repoDir, sha); !ok {
			t.Fatalf("expected to environment variables to be set")
		}

		if ok := expectAppToBeRunning(releaseName, testNamespace); !ok {
			t.Fatalf("expected the app to be running with exact content")
		}

	})
}

func expectBuiltImageNotFound(output string) bool {
	return strings.Contains(output, "image not found, building image")
}

func expectEnvSetting(output, namespace, repoDir, sha string) bool {
	rgty := okteto.Context().Registry
	repo := fmt.Sprintf("%s/%s-app", namespace, repoDir)
	img := fmt.Sprintf("%s/%s/%s-app@%s", okteto.Context().Registry, namespace, repoDir, sha)
	tag := sha
	return strings.Contains(output, fmt.Sprintf("envs registry=%s repository=%s image=%s tag=%s", rgty, repo, img, tag)) &&
		strings.Contains(output, "manifest env vars set")
}

func expectHelmInstallation(output, releaseName, namespace string) bool {
	return strings.Contains(output, fmt.Sprintf(`Release "%s" does not exist. Installing it now.`, releaseName)) &&
		strings.Contains(output, fmt.Sprintf("NAME: %s", releaseName)) &&
		strings.Contains(output, fmt.Sprintf("NAMESPACE: %s", namespace)) &&
		strings.Contains(output, "STATUS: deployed") &&
		strings.Contains(output, "REVISION: 1")
}

func expectAppToBeRunning(releaseName, namespace string) bool {
	endpoint := fmt.Sprintf("https://%s-%s.%s", releaseName, namespace, appsSubdomain)
	content, err := getContent(endpoint, 150, nil)
	if err != nil {
		log.Printf("error getting app content: %s", err.Error())
		return false
	}
	return strings.Contains(content, "Hello world!")
}

func writeFile(path, filename, content string) error {
	return ioutil.WriteFile(filepath.Join(path, filename), []byte(content), 0777)
}
