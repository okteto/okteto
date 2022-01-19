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

func TestDeployFromManifest(t *testing.T) {
	ctx := context.Background()
	oktetoPath, err := getOktetoPath(ctx)
	if err != nil {
		t.Fatal(err)
	}

	const (
		gitRepo          = "git@github.com:okteto/go-getting-started.git"
		repoDir          = "go-getting-started"
		manifestFilename = "okteto.yml"
		chartDir         = "chart"
		chartFilename    = "Chart.yaml"
		templateFilename = "k8s.yaml"
		releaseName      = "hello-world"
		manifestContent  = `build:
  app:
    context: .
deploy:
  - helm upgrade --install hello-world chart --set app.image=${build.app.image}`
		chartContent = `apiVersion: v2
name: hello-world
description: A React application in Kubernetes
type: application
version: 0.1.0
appVersion: 1.0.0`
		templateContent = `apiVersion: apps/v1
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
	)

	var (
		testID          = strings.ToLower(fmt.Sprintf("DeployFromManifest-%s-%d", runtime.GOOS, time.Now().Unix()))
		testNamespace   = fmt.Sprintf("%s-%s", testID, user)
		expectedImage   = fmt.Sprintf("%s/%s/%s-app:okteto", okteto.Context().Registry, testNamespace, repoDir)
		originNamespace = getCurrentNamespace()
	)

	if err := createNamespace(ctx, oktetoPath, testNamespace); err != nil {
		t.Fatal(err)
	}

	if err := cloneGitRepo(ctx, gitRepo); err != nil {
		t.Fatal(err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	manifestPath := filepath.Join(cwd, repoDir)
	if err := writeFile(manifestPath, manifestFilename, manifestContent); err != nil {
		t.Fatal(err)
	}

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

	t.Cleanup(func() {
		changeToNamespace(ctx, oktetoPath, originNamespace)
		deleteNamespace(ctx, oktetoPath, testNamespace)
		deleteGitRepo(ctx, repoDir)
	})

	t.Run("okteto deploy should build images if not exists at registry", func(t *testing.T) {

		output, err := runOktetoDeploy(oktetoPath, repoDir)
		if err != nil {
			t.Fatalf(err.Error())
		}

		imageWithDigest, err := registry.GetImageTagWithDigest(expectedImage)
		if err != nil {
			t.Fatalf(err.Error())
		}
		sha := strings.SplitN(imageWithDigest, "@", 2)[1]

		if err := expectBuiltImageNotFound(output); err != nil {
			t.Fatalf(err.Error())
		}

		if err := expectHelmInstallation(output, releaseName, testNamespace); err != nil {
			t.Fatalf(err.Error())
		}

		if err := expectEnvSetting(output, testNamespace, repoDir, sha); err != nil {
			t.Fatalf(err.Error())
		}

		if err := expectAppToBeRunning(releaseName, testNamespace); err != nil {
			t.Fatalf(err.Error())
		}

	})

	t.Run("okteto deploy should not build images if already at registry", func(t *testing.T) {

		imageWithDigest, err := registry.GetImageTagWithDigest(expectedImage)
		if err != nil {
			t.Fatalf("image is not at registry: %s", err.Error())
		}
		sha := strings.SplitN(imageWithDigest, "@", 2)[1]

		output, err := runOktetoDeploy(oktetoPath, repoDir)
		if err != nil {
			t.Fatalf(err.Error())
		}

		if err := expectImageFoundSkippingBuild(output); err != nil {
			t.Fatalf(err.Error())
		}

		if err := expectHelmUpgrade(output, releaseName, testNamespace, "2"); err != nil {
			t.Fatalf(err.Error())
		}

		if err := expectEnvSetting(output, testNamespace, repoDir, sha); err != nil {
			t.Fatalf(err.Error())
		}

		if err := expectAppToBeRunning(releaseName, testNamespace); err != nil {
			t.Fatalf(err.Error())
		}

	})

	t.Run("okteto deploy --build should force the build even if exist", func(t *testing.T) {

		imageWithDigest, err := registry.GetImageTagWithDigest(expectedImage)
		if err != nil {
			t.Fatalf("image is not at registry: %s", err.Error())
		}
		sha := strings.SplitN(imageWithDigest, "@", 2)[1]

		output, err := runOktetoDeployForceBuild(oktetoPath, repoDir)
		if err != nil {
			t.Fatalf(err.Error())
		}

		if err := expectForceBuild(output); err != nil {
			t.Fatalf(err.Error())
		}

		if err := expectHelmUpgrade(output, releaseName, testNamespace, "3"); err != nil {
			t.Fatalf("expected helm upgrade")
		}

		if err := expectEnvSetting(output, testNamespace, repoDir, sha); err != nil {
			t.Fatalf("expected to environment variables to be set")
		}

		if err := expectAppToBeRunning(releaseName, testNamespace); err != nil {
			t.Fatalf("expected the app to be running with exact content")
		}

	})
}

func runOktetoDeploy(oktetoPath, repoDir string) (string, error) {
	cmd := exec.Command(oktetoPath, "deploy", "-l", "debug")
	cmd.Env = append(os.Environ(), "OKTETO_ENABLE_MANIFEST_V2=true")
	cmd.Dir = repoDir
	o, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("running command: \nerror: %s \noutput: %s", err.Error(), string(o))
	}
	return string(o), nil
}

func runOktetoDeployForceBuild(oktetoPath, repoDir string) (string, error) {
	cmd := exec.Command(oktetoPath, "deploy", "--build", "-l", "debug")
	cmd.Env = append(os.Environ(), "OKTETO_ENABLE_MANIFEST_V2=true")
	cmd.Dir = repoDir
	o, err := cmd.CombinedOutput()
	if err != nil {
		return string(o), fmt.Errorf("running command: \nerror: %s \noutput: %s", err.Error(), string(o))
	}
	return string(o), nil
}

func expectBuiltImageNotFound(output string) error {
	if ok := strings.Contains(output, "image not found, building image"); !ok {
		return fmt.Errorf("expected image not found, building image")
	}
	return nil
}

func expectImageFoundSkippingBuild(output string) error {
	if ok := strings.Contains(output, "image found, skipping build"); !ok {
		return fmt.Errorf("expected image found, skipping build")
	}
	return nil
}

func expectForceBuild(output string) error {
	if ok := strings.Contains(output, "force build from manifest definition"); !ok {
		return fmt.Errorf("expected force build from manifest definition")
	}
	return nil
}

func expectEnvSetting(output, namespace, repoDir, sha string) error {
	rgty := okteto.Context().Registry
	repo := fmt.Sprintf("%s/%s-app", namespace, repoDir)
	img := fmt.Sprintf("%s/%s/%s-app@%s", okteto.Context().Registry, namespace, repoDir, sha)

	if ok := strings.Contains(output, fmt.Sprintf("envs registry=%s repository=%s image=%s tag=%s", rgty, repo, img, sha)) &&
		strings.Contains(output, "manifest env vars set"); !ok {
		return fmt.Errorf("expected envs registry=%s repository=%s image=%s tag=%s", rgty, repo, img, sha)
	}
	return nil
}

func expectHelmInstallation(output, releaseName, namespace string) error {
	if ok := strings.Contains(output, fmt.Sprintf(`Release "%s" does not exist. Installing it now.`, releaseName)) &&
		strings.Contains(output, fmt.Sprintf("NAME: %s", releaseName)) &&
		strings.Contains(output, fmt.Sprintf("NAMESPACE: %s", namespace)) &&
		strings.Contains(output, "STATUS: deployed") &&
		strings.Contains(output, "REVISION: 1"); !ok {
		return fmt.Errorf("expected helm chart to be installed")
	}
	return nil
}

func expectHelmUpgrade(output, releaseName, namespace, revision string) error {
	if ok := strings.Contains(output, fmt.Sprintf(`Release "%s" has been upgraded.`, releaseName)) &&
		strings.Contains(output, fmt.Sprintf("NAME: %s", releaseName)) &&
		strings.Contains(output, fmt.Sprintf("NAMESPACE: %s", namespace)) &&
		strings.Contains(output, "STATUS: deployed") &&
		strings.Contains(output, fmt.Sprintf("REVISION: %s", revision)); !ok {
		return fmt.Errorf("expected helm chart to be upgraded to revision %s", revision)
	}
	return nil
}

func expectAppToBeRunning(releaseName, namespace string) error {
	endpoint := fmt.Sprintf("https://%s-%s.%s", releaseName, namespace, appsSubdomain)
	content, err := getContent(endpoint, 150, nil)
	if err != nil {
		return err
	}
	if ok := strings.Contains(content, "Hello world!"); !ok {
		return fmt.Errorf("expected app content")
	}
	return nil
}
