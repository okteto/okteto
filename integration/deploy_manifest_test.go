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
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	v1 "k8s.io/api/apps/v1"
)

type helmRelease struct {
	Revision int                    `json:"revision"`
	Status   string                 `json:"status"`
	Other    map[string]interface{} `json:"-"`
}

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

		manifestContent = `
build:
  app:
    context: .
deploy:
  - helm upgrade --install hello-world chart --set app.image=${OKTETO_BUILD_APP_IMAGE}`

		chartContent = `
apiVersion: v2
name: hello-world
description: A React application in Kubernetes
type: application
version: 0.1.0
appVersion: 1.0.0`

		templateContent = `
apiVersion: apps/v1
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

	if err := createNamespace(ctx, oktetoPath, testNamespace); err != nil {
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
			t.Fatal(err)
		}

		imageWithDigest, err := registry.GetImageTagWithDigest(expectedImage)
		if err != nil {
			t.Fatal(err)
		}
		sha := strings.SplitN(imageWithDigest, "@", 2)[1]

		d, err := getDeployment(ctx, testNamespace, releaseName)
		if err != nil {
			t.Fatal(err)
		}

		if err := expectDeployment(ctx, d, []string{imageWithDigest}, 1); err != nil {
			t.Fatal(err)
		}

		if err := expectBuiltImageNotFound(output); err != nil {
			t.Fatal(err)
		}

		if err := expectHelm(output, releaseName, testNamespace, 1); err != nil {
			t.Fatal(err)
		}

		if err := expectEnvSetting(output, testNamespace, repoDir, sha); err != nil {
			t.Fatal(err)
		}

	})

	t.Run("okteto deploy should not build images if already at registry", func(t *testing.T) {

		imageWithDigest, err := registry.GetImageTagWithDigest(expectedImage)
		if err != nil {
			t.Fatalf("image is not at registry: %v", err)
		}
		sha := strings.SplitN(imageWithDigest, "@", 2)[1]

		output, err := runOktetoDeploy(oktetoPath, repoDir)
		if err != nil {
			t.Fatal(err)
		}

		d, err := getDeployment(ctx, testNamespace, releaseName)
		if err != nil {
			t.Fatal(err)
		}

		if err := expectDeployment(ctx, d, []string{imageWithDigest}, 1); err != nil {
			t.Fatal(err)
		}

		if err := expectImageFoundSkippingBuild(output); err != nil {
			t.Fatal(err)
		}

		if err := expectHelm(output, releaseName, testNamespace, 2); err != nil {
			t.Fatal(err)
		}

		if err := expectEnvSetting(output, testNamespace, repoDir, sha); err != nil {
			t.Fatal(err)
		}

	})

	t.Run("okteto deploy --build should force the build an image does not change if no code changes", func(t *testing.T) {

		imageWithDigest, err := registry.GetImageTagWithDigest(expectedImage)
		if err != nil {
			t.Fatalf("image is not at registry: %v", err)
		}
		originalSHA := strings.SplitN(imageWithDigest, "@", 2)[1]

		output, err := runOktetoDeployForceBuild(oktetoPath, repoDir)
		if err != nil {
			t.Fatal(err)
		}

		d, err := getDeployment(ctx, testNamespace, releaseName)
		if err != nil {
			t.Fatal(err)
		}

		if err := expectDeployment(ctx, d, []string{imageWithDigest}, 1); err != nil {
			t.Fatal(err)
		}

		if err := expectForceBuild(output); err != nil {
			t.Fatal(err)
		}

		if err := expectHelm(output, releaseName, testNamespace, 3); err != nil {
			t.Fatal(err)
		}

		if err := expectEnvSetting(output, testNamespace, repoDir, originalSHA); err != nil {
			t.Fatal(err)
		}

	})

	t.Run("okteto deploy --build should force the build an image change if code changes", func(t *testing.T) {
		imageWithDigest, err := registry.GetImageTagWithDigest(expectedImage)
		if err != nil {
			t.Fatalf("image is not at registry: %v", err)
		}
		originalSHA := strings.SplitN(imageWithDigest, "@", 2)[1]

		mainFile := filepath.Join(cwd, repoDir, "main.go")
		mainFileContent, err := ioutil.ReadFile(mainFile)
		if err != nil {
			t.Fatal(err)
		}
		updatedMainFileContent := strings.Replace(string(mainFileContent), "Hello", "Bye", 1)
		if err := writeFile(filepath.Join(cwd, repoDir), "main.go", updatedMainFileContent); err != nil {
			t.Fatal(err)
		}

		output, err := runOktetoDeployForceBuild(oktetoPath, repoDir)
		if err != nil {
			t.Fatal(err)
		}

		newImageWithDigest, err := registry.GetImageTagWithDigest(expectedImage)
		if err != nil {
			t.Fatalf("image is not at registry: %v", err)
		}
		newSHA := strings.SplitN(newImageWithDigest, "@", 2)[1]

		if originalSHA == newSHA {
			t.Fatal("image has not been updated")
		}

		d, err := getDeployment(ctx, testNamespace, releaseName)
		if err != nil {
			t.Fatal(err)
		}

		if err := expectDeployment(ctx, d, []string{newImageWithDigest}, 2); err != nil {
			t.Fatal(err)
		}

		if err := expectForceBuild(output); err != nil {
			t.Fatal(err)
		}

		if err := expectHelm(output, releaseName, testNamespace, 4); err != nil {
			t.Fatal(err)
		}

		if err := expectEnvSetting(output, testNamespace, repoDir, newSHA); err != nil {
			t.Fatal(err)
		}

	})
}

func runOktetoDeploy(oktetoPath, repoDir string) (string, error) {
	cmd := exec.Command(oktetoPath, "deploy", "-l", "debug")
	cmd.Dir = repoDir
	o, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("okteto deploy failed: \nerror: %s \noutput: %s", err.Error(), string(o))
	}
	return string(o), nil
}

func runOktetoDeployForceBuild(oktetoPath, repoDir string) (string, error) {
	cmd := exec.Command(oktetoPath, "deploy", "--build", "-l", "debug")
	cmd.Dir = repoDir
	o, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("okteto deploy --build failed: \nerror: %s \noutput: %s", err.Error(), string(o))
	}
	return string(o), nil
}

func expectBuiltImageNotFound(output string) error {
	if ok := strings.Contains(output, "image not found, building image"); !ok {
		return errors.New("expected image not found, building image")
	}
	return nil
}

func expectImageFoundSkippingBuild(output string) error {
	if ok := strings.Contains(output, "Skipping build for image for service"); !ok {
		return errors.New("expected image found, skipping build")
	}
	return nil
}

func expectForceBuild(output string) error {
	if ok := strings.Contains(output, "force build from manifest definition"); !ok {
		return errors.New("expected force build from manifest definition")
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

func expectHelm(output, releaseName, namespace string, revision int) error {
	cmd := exec.Command("helm", "history", releaseName, "-n", namespace, "-o", "json")
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("helm history failed: \nerror: %s \noutput: %s", err.Error(), string(o))
	}
	helmHistoryOutput := []helmRelease{}
	if err := json.Unmarshal(o, &helmHistoryOutput); err != nil {
		return fmt.Errorf("could not parse %s: %s", string(o), err.Error())
	}
	if len(helmHistoryOutput) != revision {
		return fmt.Errorf("Wrong number of releases: Expected %d but got %d", revision, len(helmHistoryOutput))
	}
	helmRelease := helmHistoryOutput[revision-1]
	if helmRelease.Revision != revision || helmRelease.Status != "deployed" {
		return fmt.Errorf("wrong helm release: %v", helmRelease)
	}
	return nil
}

func expectAppToBeRunning(releaseName, namespace, contentString string) error {
	endpoint := fmt.Sprintf("https://%s-%s.%s", releaseName, namespace, appsSubdomain)
	content, err := getContent(endpoint, 150, nil)
	if err != nil {
		return err
	}
	if ok := strings.Contains(content, contentString); !ok {
		return fmt.Errorf("expected app content to be %s", contentString)
	}
	return nil
}

func expectDeployment(ctx context.Context, d *v1.Deployment, images []string, revision int64) error {
	dRev := d.ObjectMeta.Generation
	if dRev != revision {
		return fmt.Errorf("expected revision %d, got %d", revision, dRev)
	}

	containers := d.Spec.Template.Spec.Containers
	gotContainers := len(containers)
	expContainers := len(images)
	if len(containers) != len(images) {
		return fmt.Errorf("expected number of containers %d, got %d", expContainers, gotContainers)
	}

	found := 0
	for _, i := range images {
		for _, c := range containers {
			if c.Image == i {
				found++
			}
		}
	}
	if found != len(images) {
		return fmt.Errorf("expected images to match container images, found %d", found)
	}
	return nil

}
