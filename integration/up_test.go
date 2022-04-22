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
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"text/template"
	"time"

	"github.com/Masterminds/semver/v3"
	ps "github.com/mitchellh/go-ps"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	"github.com/okteto/okteto/pkg/k8s/statefulsets"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/syncthing"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	yaml "gopkg.in/yaml.v2"
)

var (
	deploymentTemplate               = template.Must(template.New("deployment").Parse(deploymentFormat))
	statefulsetTemplate              = template.Must(template.New("statefulset").Parse(statefulsetFormat))
	manifestTemplate                 = template.Must(template.New("manifest").Parse(manifestFormat))
	autocreateManifestTemplate       = template.Must(template.New("manifest").Parse(autocreateManifestFormat))
	zero                       int64 = 0
)

const (
	indexEndpoint    = "http://localhost:8080/index.html"
	varEndpoint      = "http://localhost:8080/var.html"
	deploymentFormat = `
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
      terminationGracePeriodSeconds: 1
      containers:
      - name: test
        image: python:alpine
        ports:
        - containerPort: 8080
        workingDir: /usr/src/app
        env:
          - name: VAR
            value: value1
        command:
            - sh
            - -c
            - "echo -n $VAR > var.html && python -m http.server 8080"
---
apiVersion: v1
kind: Service
metadata:
  name: {{ .Name }}
  annotations:
    dev.okteto.com/auto-ingress: "true"
spec:
  type: ClusterIP
  ports:
  - name: {{ .Name }}
    port: 8080
  selector:
    app: {{ .Name }}
`
	statefulsetFormat = `
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: {{ .Name }}
spec:
  serviceName: {{ .Name }}
  replicas: 1
  selector:
    matchLabels:
      app: {{ .Name }}
  template:
    metadata:
      labels:
        app: {{ .Name }}
    spec:
      terminationGracePeriodSeconds: 1
      containers:
      - name: test
        image: python:alpine
        ports:
        - containerPort: 8080
        workingDir: /usr/src/app
        env:
          - name: VAR
            value: value1
        command:
            - sh
            - -c
            - "echo -n $VAR > var.html && python -m http.server 8080"
---
apiVersion: v1
kind: Service
metadata:
  name: {{ .Name }}
  annotations:
    dev.okteto.com/auto-ingress: "true"
spec:
  type: ClusterIP
  ports:
  - name: {{ .Name }}
    port: 8080
  selector:
    app: {{ .Name }}
`
	manifestFormat = `
name: {{ .Name }}
image: python:alpine
command:
  - sh
  - -c
  - "echo -n $VAR > var.html && python -m http.server 8080"
forward:
  - 8080:8080
workdir: /usr/src/app
persistentVolume:
  enabled: false
`
	autocreateManifestFormat = `
name: {{ .Name }}
autocreate: true
image: python:alpine
command:
  - sh
  - -c
  - "echo -n $VAR > var.html && python -m http.server 8080"
forward:
  - 8080:8080
workdir: /usr/src/app
persistentVolume:
  enabled: false
environment:
  VAR: value2
`
	divertGitRepo              = "git@github.com:okteto/catalog.git"
	divertGitFolder            = "catalog"
	microservicesComposeRepo   = "https://github.com/okteto/microservices-demo-compose"
	microservicesComposeFolder = "microservices-demo-compose"
)

var mode string

func TestGetVersion(t *testing.T) {
	v, err := utils.GetLatestVersionFromGithub()
	if err != nil {
		t.Fatal(err)
	}

	_, err = semver.NewVersion(v)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDownloadSyncthing(t *testing.T) {
	var tests = []struct {
		os   string
		arch string
	}{
		{os: "windows", arch: "amd64"},
		{os: "darwin", arch: "amd64"},
		{os: "darwin", arch: "arm64"},
		{os: "linux", arch: "amd64"},
		{os: "linux", arch: "arm64"},
		{os: "linux", arch: "arm"},
	}

	ctx := context.Background()
	m := syncthing.GetMinimumVersion()
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s-%s", tt.os, tt.arch), func(t *testing.T) {
			u, err := syncthing.GetDownloadURL(tt.os, tt.arch, m.String())
			req, err := http.NewRequest("GET", u, nil)
			if err != nil {
				t.Fatal(err.Error())
			}

			req = req.WithContext(ctx)
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Failed to download syncthing: %s", err)
			}

			if res.StatusCode != 200 {
				t.Fatalf("Failed to download syncthing. Got status: %d", res.StatusCode)
			}
		})
	}
}

func TestUpDeployments(t *testing.T) {
	tName := fmt.Sprintf("TestAll-%s-%s", runtime.GOOS, mode)
	ctx := context.Background()
	oktetoPath, err := getOktetoPath(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := exec.LookPath(kubectlBinary); err != nil {
		t.Fatalf("kubectl is not in the path: %s", err)
	}

	name := strings.ToLower(fmt.Sprintf("%s-%d", tName, time.Now().Unix()))
	namespace := fmt.Sprintf("%s-%s", name, user)

	dir, err := os.MkdirTemp("", tName)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	log.Printf("created tempdir: %s", dir)

	dPath := filepath.Join(dir, "deployment.yaml")
	if err := writeDeployment(deploymentTemplate, name, dPath); err != nil {
		t.Fatal(err)
	}

	contentPath := filepath.Join(dir, "index.html")
	if err := os.WriteFile(contentPath, []byte(name), 0644); err != nil {
		t.Fatal(err)
	}

	log.Printf("original content: %s", name)

	manifestPath := filepath.Join(dir, "okteto.yml")
	if err := writeManifest(manifestPath, name); err != nil {
		t.Fatal(err)
	}

	stignorePath := filepath.Join(dir, ".stignore")
	if err := os.WriteFile(stignorePath, []byte("venv"), 0600); err != nil {
		t.Fatal(err)
	}

	startNamespace := getCurrentNamespace()
	defer changeToNamespace(ctx, oktetoPath, startNamespace)
	if err := createNamespace(ctx, oktetoPath, namespace); err != nil {
		t.Fatal(err)
	}
	defer deleteNamespace(ctx, oktetoPath, namespace)

	if err := deploy(ctx, namespace, name, dPath, true); err != nil {
		t.Fatal(err)
	}

	originalDeployment, err := getDeployment(ctx, namespace, name)
	if err != nil {
		t.Fatal(err)
	}

	log.Printf("deployment: %s, revision: %s", originalDeployment.Name, originalDeployment.Annotations[model.DeploymentRevisionAnnotation])

	var wg sync.WaitGroup
	upErrorChannel := make(chan error, 1)
	p, err := up(ctx, &wg, namespace, name, manifestPath, oktetoPath, upErrorChannel)
	if err != nil {
		t.Fatal(err)
	}

	waitForDeployment(ctx, namespace, name, 2, 300)

	defer showUpLogs(name, namespace, t)

	log.Println("getting synchronized content")

	content, err := getContent(indexEndpoint, 300, upErrorChannel)
	if err != nil {
		t.Fatalf("failed to get index content: %s", err)
	}

	log.Println("got synchronized index content")

	if content != name {
		t.Fatalf("expected synchronized index content to be '%s', got '%s'", name, content)
	}

	content, err = getContent(varEndpoint, 300, upErrorChannel)
	if err != nil {
		t.Fatalf("failed to get var content: %s", err)
	}

	log.Println("got synchronized var content")

	if content != "value1" {
		t.Fatalf("expected var content to be 'value1', got '%s'", content)
	}

	if err := testRemoteStignoreGenerated(ctx, namespace, name, manifestPath, oktetoPath); err != nil {
		t.Fatal(err)
	}

	c, _, err := K8sClient()
	if err != nil {
		t.Fatal(err)
	}
	d, err := getDeployment(ctx, namespace, name)
	if err != nil {
		t.Fatal(err)
	}
	originalDeployment.Spec.Template.Spec.Containers[0].Env[0].Value = "value2"
	d.Spec.Template.Spec.Containers[0].Env[0].Value = "value2"
	if _, err := deployments.Deploy(ctx, d, c); err != nil {
		t.Fatal(err)
	}

	if err := testUpdateContent(fmt.Sprintf("%s-updated", name), contentPath, 300, upErrorChannel); err != nil {
		t.Fatal(err)
	}

	if err := killLocalSyncthing(); err != nil {
		t.Fatal(err)
	}

	if err := testUpdateContent(fmt.Sprintf("%s-kill-syncthing", name), contentPath, 300, upErrorChannel); err != nil {
		t.Fatal(err)
	}

	if err := destroyPod(ctx, name, namespace); err != nil {
		t.Fatal(err)
	}

	if err := testUpdateContent(fmt.Sprintf("%s-destroy-pod", name), contentPath, 300, upErrorChannel); err != nil {
		t.Fatal(err)
	}

	initialPod, err := getPod(ctx, c, name, namespace)
	if err != nil {
		t.Fatal(err)
	}

	d, err = getDeployment(ctx, namespace, name)
	if err != nil {
		t.Fatal(err)
	}

	log.Printf("deployment: %s, revision: %s", d.Name, d.Annotations[model.DeploymentRevisionAnnotation])

	var newWg sync.WaitGroup
	newUpErrorChannel := make(chan error, 1)

	newProccess, err := up(ctx, &newWg, namespace, name, manifestPath, oktetoPath, newUpErrorChannel)
	if err != nil {
		t.Fatal(err)
	}

	if err := testUpdateContent(fmt.Sprintf("%s-reconnect", name), contentPath, 300, newUpErrorChannel); err != nil {
		t.Fatal(err)
	}

	if err := checkIfUpFinished(ctx, p.Pid); err != nil {
		t.Error(err)
	}

	newPod, err := getPod(ctx, c, name, namespace)
	if err != nil {
		t.Fatal(err)
	}

	if initialPod.Name != newPod.Name {
		t.Fatalf("expected pods to have the same name: %s - %s", initialPod.Name, newPod.Name)
	}

	if err := down(ctx, namespace, name, manifestPath, oktetoPath, true, false); err != nil {
		t.Fatal(err)
	}

	if err := checkIfUpFinished(ctx, newProccess.Pid); err != nil {
		t.Error(err)
	}

	if err := compareDeployment(ctx, originalDeployment); err != nil {
		t.Error(err)
	}

}
func TestUpStatefulset(t *testing.T) {

	tName := fmt.Sprintf("TestAllSfs-%s-%s", runtime.GOOS, mode)
	ctx := context.Background()
	oktetoPath, err := getOktetoPath(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := exec.LookPath(kubectlBinary); err != nil {
		t.Fatalf("kubectl is not in the path: %s", err)
	}

	name := strings.ToLower(fmt.Sprintf("%s-%d", tName, time.Now().Unix()))
	namespace := fmt.Sprintf("%s-%s", name, user)

	dir, err := os.MkdirTemp("", tName)
	if err != nil {
		t.Fatal(err)
	}
	log.Printf("created tempdir: %s", dir)

	sfsPath := filepath.Join(dir, "statefulset.yaml")
	if err := writeStatefulset(name, sfsPath); err != nil {
		t.Fatal(err)
	}

	contentPath := filepath.Join(dir, "index.html")
	if err := os.WriteFile(contentPath, []byte(name), 0644); err != nil {
		t.Fatal(err)
	}

	log.Printf("original content: %s", name)

	manifestPath := filepath.Join(dir, "okteto.yml")
	if err := writeManifest(manifestPath, name); err != nil {
		t.Fatal(err)
	}

	stignorePath := filepath.Join(dir, ".stignore")
	if err := os.WriteFile(stignorePath, []byte("venv"), 0600); err != nil {
		t.Fatal(err)
	}

	startNamespace := getCurrentNamespace()
	defer changeToNamespace(ctx, oktetoPath, startNamespace)
	if err := createNamespace(ctx, oktetoPath, namespace); err != nil {
		t.Fatal(err)
	}

	if err := deploy(ctx, namespace, name, sfsPath, false); err != nil {
		t.Fatal(err)
	}

	originalStatefulset, err := getStatefulset(ctx, namespace, name)
	if err != nil {
		t.Fatal(err)
	}

	log.Printf("statefulset: %s, revision: %s", originalStatefulset.Name, originalStatefulset.Labels[appsv1.StatefulSetRevisionLabel])

	var wg sync.WaitGroup
	upErrorChannel := make(chan error, 1)
	p, err := up(ctx, &wg, namespace, name, manifestPath, oktetoPath, upErrorChannel)
	if err != nil {
		t.Fatal(err)
	}

	waitForStatefulset(ctx, namespace, name, 300)

	defer showUpLogs(name, namespace, t)

	log.Println("getting synchronized content")

	content, err := getContent(indexEndpoint, 300, upErrorChannel)
	if err != nil {
		t.Fatalf("failed to get content: %s", err)
	}
	log.Println("got synchronized content")

	if content != name {
		t.Fatalf("expected synchronized content to be %s, got %s", name, content)
	}

	content, err = getContent(varEndpoint, 300, upErrorChannel)
	if err != nil {
		t.Fatalf("failed to get var content: %s", err)
	}

	log.Println("got synchronized var content")

	if content != "value1" {
		t.Fatalf("expected var content to be 'value1', got '%s'", content)
	}

	if err := testRemoteStignoreGenerated(ctx, namespace, name, manifestPath, oktetoPath); err != nil {
		t.Fatal(err)
	}

	c, _, err := K8sClient()
	if err != nil {
		t.Fatal(err)
	}
	originalStatefulset.Spec.Template.Spec.Containers[0].Env[0].Value = "value2"
	sfs, err := getStatefulset(ctx, namespace, name)
	if err != nil {
		t.Fatal(err)
	}
	sfs.Spec.Template.Spec.Containers[0].Env[0].Value = "value2"
	if _, err := statefulsets.Deploy(ctx, sfs, c); err != nil {
		t.Fatal(err)
	}

	if err := testUpdateContent(fmt.Sprintf("%s-updated", name), contentPath, 300, upErrorChannel); err != nil {
		t.Fatal(err)
	}

	if err := killLocalSyncthing(); err != nil {
		t.Fatal(err)
	}

	if err := testUpdateContent(fmt.Sprintf("%s-kill-syncthing", name), contentPath, 300, upErrorChannel); err != nil {
		t.Fatal(err)
	}

	if err := destroyPod(ctx, name, namespace); err != nil {
		t.Fatal(err)
	}

	if err := testUpdateContent(fmt.Sprintf("%s-destroy-pod", name), contentPath, 300, upErrorChannel); err != nil {
		t.Fatal(err)
	}

	sfs, err = getStatefulset(ctx, namespace, name)
	if err != nil {
		t.Fatal(err)
	}

	log.Printf("statefulset: %s", sfs.Name)

	if err := down(ctx, namespace, name, manifestPath, oktetoPath, false, false); err != nil {
		t.Fatal(err)
	}

	if err := checkIfUpFinished(ctx, p.Pid); err != nil {
		t.Error(err)
	}

	if err := compareStatefulset(ctx, originalStatefulset); err != nil {
		t.Error(err)
	}

	if err := deleteNamespace(ctx, oktetoPath, namespace); err != nil {
		log.Printf("failed to delete namespace %s: %s\n", namespace, err)
	}

}

func TestDivert(t *testing.T) {
	tName := fmt.Sprintf("Test")
	ctx := context.Background()
	oktetoPath, err := getOktetoPath(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := exec.LookPath(kubectlBinary); err != nil {
		t.Fatalf("kubectl is not in the path: %s", err)
	}
	name := strings.ToLower(fmt.Sprintf("%s-%d", tName, time.Now().Unix()))
	namespace := fmt.Sprintf("%s-%s", name, user)
	log.Printf("running %s \n", tName)
	startNamespace := getCurrentNamespace()
	defer changeToNamespace(ctx, oktetoPath, startNamespace)

	if err := createNamespace(ctx, oktetoPath, namespace); err != nil {
		t.Fatal(err)
	}

	log.Printf("created namespace %s \n", namespace)

	if err := cloneGitRepo(ctx, divertGitRepo); err != nil {
		t.Fatal(err)
	}
	defer deleteGitRepo(ctx, divertGitFolder)

	log.Printf("cloned repo %s \n", divertGitRepo)

	defer deleteGitRepo(ctx, pushGitFolder)

	if err := oktetoPipeline(ctx, oktetoPath, divertGitRepo, divertGitFolder); err != nil {
		t.Fatal(err)
	}

	log.Printf("pipeline using %s \n", divertGitRepo)

	waitForDeployment(ctx, namespace, "health-checker", 1, 300)

	if err := modifyDivertApp(); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	upErrorChannel := make(chan error, 1)
	p, err := up(ctx, &wg, namespace, fmt.Sprintf("health-checker-%s", user), filepath.Join(divertGitFolder, "health-checker", "okteto.yml"), oktetoPath, upErrorChannel)
	if err != nil {
		t.Fatal(err)
	}

	divertedSvcName := fmt.Sprintf("health-checker-%s-okteto", user)
	waitForDeployment(ctx, namespace, divertedSvcName, 2, 300)

	defer showUpLogs(name, namespace, t)

	apiSvc := "catalog-chart"
	originalContent, err := getContent(fmt.Sprintf("https://%s-%s.%s/data", apiSvc, namespace, appsSubdomain), 300, upErrorChannel)
	if err != nil {
		t.Fatal(err)
	}

	if err := waitForDivertedContent(originalContent, namespace, apiSvc, upErrorChannel, 300); err != nil {

		t.Fatal("Contents are the same")
	}

	if err := down(ctx, namespace, fmt.Sprintf("health-checker-%s", user), filepath.Join(divertGitFolder, "health-checker", "okteto.yml"), oktetoPath, false, true); err != nil {
		t.Fatal(err)
	}

	if err := checkIfUpFinished(ctx, p.Pid); err != nil {
		t.Error(err)
	}

	_, err = getDeployment(ctx, namespace, fmt.Sprintf("health-checker-%s", user))
	if err == nil {
		t.Fatalf("'dev' deployment not deleted after 'okteto stack destroy'")
	}

	if err := deleteNamespace(ctx, oktetoPath, namespace); err != nil {
		log.Printf("failed to delete namespace %s: %s\n", namespace, err)
	}
}

func TestUpAutocreate(t *testing.T) {
	tName := fmt.Sprintf("TestAutocreate-%s-%s", runtime.GOOS, mode)
	ctx := context.Background()
	oktetoPath, err := getOktetoPath(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := exec.LookPath(kubectlBinary); err != nil {
		t.Fatalf("kubectl is not in the path: %s", err)
	}

	name := strings.ToLower(fmt.Sprintf("%s-%d", tName, time.Now().Unix()))
	namespace := fmt.Sprintf("%s-%s", name, user)

	dir, err := os.MkdirTemp("", tName)
	if err != nil {
		t.Fatal(err)
	}
	log.Printf("created tempdir: %s", dir)

	contentPath := filepath.Join(dir, "index.html")
	if err := os.WriteFile(contentPath, []byte(name), 0644); err != nil {
		t.Fatal(err)
	}

	log.Printf("original content: %s", name)

	manifestPath := filepath.Join(dir, "okteto.yml")
	if err := writeAutocreateManifest(manifestPath, name); err != nil {
		t.Fatal(err)
	}

	stignorePath := filepath.Join(dir, ".stignore")
	if err := os.WriteFile(stignorePath, []byte("venv"), 0600); err != nil {
		t.Fatal(err)
	}

	startNamespace := getCurrentNamespace()
	defer changeToNamespace(ctx, oktetoPath, startNamespace)
	if err := createNamespace(ctx, oktetoPath, namespace); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	upErrorChannel := make(chan error, 1)
	p, err := up(ctx, &wg, namespace, name, manifestPath, oktetoPath, upErrorChannel)
	if err != nil {
		t.Fatal(err)
	}

	waitForDeployment(ctx, namespace, fmt.Sprintf("%s-okteto", name), 1, 300)

	defer showUpLogs(name, namespace, t)

	log.Println("getting synchronized content")

	content, err := getContent(indexEndpoint, 300, upErrorChannel)
	if err != nil {
		t.Fatalf("failed to get index content: %s", err)
	}

	log.Println("got synchronized index content")

	if content != name {
		t.Fatalf("expected synchronized index content to be '%s', got '%s'", name, content)
	}

	if err := testRemoteStignoreGenerated(ctx, namespace, name, manifestPath, oktetoPath); err != nil {
		t.Fatal(err)
	}

	if err := killLocalSyncthing(); err != nil {
		t.Fatal(err)
	}

	if err := testUpdateContent(fmt.Sprintf("%s-kill-syncthing", name), contentPath, 300, upErrorChannel); err != nil {
		t.Fatal(err)
	}

	if err := destroyPod(ctx, name, namespace); err != nil {
		t.Fatal(err)
	}

	if err := testUpdateContent(fmt.Sprintf("%s-destroy-pod", name), contentPath, 300, upErrorChannel); err != nil {
		t.Fatal(err)
	}

	d, err := getDeployment(ctx, namespace, fmt.Sprintf("%s-okteto", name))
	if err != nil {
		t.Fatal(err)
	}

	log.Printf("deployment: %s, revision: %s", d.Name, d.Annotations[model.DeploymentRevisionAnnotation])

	if err := down(ctx, namespace, name, manifestPath, oktetoPath, true, true); err != nil {
		t.Fatal(err)
	}

	if err := checkIfUpFinished(ctx, p.Pid); err != nil {
		t.Error(err)
	}

	if err := deleteNamespace(ctx, oktetoPath, namespace); err != nil {
		log.Printf("failed to delete namespace %s: %s\n", namespace, err)
	}
}

func TestUpCompose(t *testing.T) {
	tName := fmt.Sprintf("TestUpCompose-%s-%s", runtime.GOOS, mode)
	ctx := context.Background()
	oktetoPath, err := getOktetoPath(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := exec.LookPath(kubectlBinary); err != nil {
		t.Fatalf("kubectl is not in the path: %s", err)
	}
	name := strings.ToLower(fmt.Sprintf("%s-%d", tName, time.Now().Unix()))
	namespace := fmt.Sprintf("%s-%s", name, user)
	log.Printf("running %s \n", tName)
	startNamespace := getCurrentNamespace()
	defer changeToNamespace(ctx, oktetoPath, startNamespace)

	if err := createNamespace(ctx, oktetoPath, namespace); err != nil {
		t.Fatal(err)
	}

	log.Printf("created namespace %s \n", namespace)

	if err := cloneGitRepo(ctx, microservicesComposeRepo); err != nil {
		t.Fatal(err)
	}
	defer deleteGitRepo(ctx, microservicesComposeRepo)

	log.Printf("cloned repo %s \n", microservicesComposeRepo)

	defer deleteGitRepo(ctx, microservicesComposeFolder)

	var wg sync.WaitGroup
	upErrorChannel := make(chan error, 1)
	p, err := upWithSvc(ctx, &wg, namespace, microservicesComposeFolder, "vote", oktetoPath, upErrorChannel)
	if err != nil {
		t.Fatal(err)
	}

	svc, err := getService(ctx, namespace, "vote")
	if err != nil {
		t.Fatal(err)
	}

	if len(svc.Spec.Ports) != 1 {
		t.Fatalf("Expected to have only one endpoint for svc 'vote' but got %d", len(svc.Spec.Ports))
	}

	ingress, err := getIngress(ctx, namespace, "okteto-vote")
	if err != nil {
		t.Fatal(err)
	}

	for _, rule := range ingress.Spec.Rules {
		for _, path := range rule.HTTP.Paths {
			svc := path.Backend.Service
			if svc.Name != "vote" {
				t.Fatalf("ingress is referencing other service: %s", svc.Name)
			}
			if svc.Port.Number == 5005 {
				t.Fatal("Didn't expect a debugger to have an endpoint")
			}
		}
	}
	if len(svc.Spec.Ports) != 1 {
		t.Fatalf("Expected to have only one endpoint for svc 'vote' but got %d", len(svc.Spec.Ports))
	}

	port := "5005"
	ln, err := net.Listen("tcp", "localhost:"+port)

	if err == nil {
		_ = ln.Close()
		t.Fatalf("port 5005 is available locally")
	}

	if err := downSvc(ctx, "vote", microservicesComposeFolder, oktetoPath); err != nil {
		t.Fatal(err)
	}

	if err := checkIfUpFinished(ctx, p.Pid); err != nil {
		t.Error(err)
	}

	if err := deleteNamespace(ctx, oktetoPath, namespace); err != nil {
		log.Printf("failed to delete namespace %s: %s\n", namespace, err)
	}

}

func oktetoPipeline(ctx context.Context, oktetoPath, repo, folder string) error {
	log.Printf("okteto pipeline --wait")
	cmd := exec.Command(oktetoPath, "pipeline", "deploy", "--wait")
	cmd.Env = os.Environ()
	cmd.Dir = folder
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("okteto pipeline failed: %s - %s", string(o), err)
	}
	log.Printf("okteto pipeline %s success", repo)
	return nil
}

func modifyDivertApp() error {

	input, err := os.ReadFile(filepath.Join(divertGitFolder, "health-checker", "cmd", "main.go"))
	if err != nil {
		return err
	}

	output := bytes.Replace(input, []byte("&health.SimpleHealthClient"), []byte("&health.AdvancedHealthClient"), 1)

	if err = os.WriteFile(filepath.Join(divertGitFolder, "health-checker", "cmd", "main.go"), output, 0666); err != nil {
		return err
	}

	input, err = os.ReadFile(filepath.Join(divertGitFolder, "health-checker", "okteto.yml"))
	if err != nil {
		return err
	}

	output = bytes.Replace(input, []byte("command: bash"), []byte("command: go run cmd/main.go"), 1)

	if err = os.WriteFile(filepath.Join(divertGitFolder, "health-checker", "okteto.yml"), output, 0666); err != nil {
		return err
	}
	return nil
}

func showUpLogs(name, namespace string, t *testing.T) {
	if !t.Failed() {
		return
	}
	logsPath := filepath.Join(config.GetAppHome(namespace, name), "okteto.log")
	logBytes, err := os.ReadFile(logsPath)
	if err == nil {
		fmt.Println("up logs:", string(logBytes))
	}
}

func waitForDivertedContent(originalContent, namespace, apiSvc string, upErrorChannel chan error, timeout int) error {
	for i := 0; i < timeout; i++ {
		apiDivertedSvc := fmt.Sprintf("%s-%s", apiSvc, user)
		divertedContent, err := getContent(fmt.Sprintf("https://%s-%s.%s/data", apiDivertedSvc, namespace, appsSubdomain), 3, upErrorChannel)
		if err != nil {
			continue
		}

		if originalContent != divertedContent {
			return nil
		}
	}
	return fmt.Errorf("Content was not diverted")
}

func waitForDeployment(ctx context.Context, namespace, name string, revision, timeout int) error {
	for i := 0; i < timeout; i++ {
		args := []string{"--namespace", namespace, "rollout", "status", "deployment", name, "--revision", fmt.Sprintf("%d", revision)}

		cmd := exec.Command(kubectlBinary, args...)
		cmd.Env = os.Environ()
		o, _ := cmd.CombinedOutput()
		log.Printf("waitForDeployment command: %s", cmd.String())
		output := string(o)
		log.Printf("waitForDeployment output: %s", output)

		if strings.Contains(output, "is different from the running revision") {
			r := regexp.MustCompile("\\(\\d+\\)")
			matches := r.FindAllString(output, -1)
			if len(matches) == 2 {
				desiredVersion := strings.ReplaceAll(strings.ReplaceAll(matches[0], "(", ""), ")", "")
				runningVersion := strings.ReplaceAll(strings.ReplaceAll(matches[0], "(", ""), ")", "")
				if desiredVersion <= runningVersion {
					log.Println(output)
					return nil
				}
			}
			time.Sleep(1 * time.Second)
			continue
		}

		if strings.Contains(output, "successfully rolled out") {
			log.Println(output)
			return nil
		}

		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("%s didn't rollout after %d seconds", name, timeout)
}

func waitForStatefulset(ctx context.Context, namespace, name string, timeout int) error {
	for i := 0; i < timeout; i++ {
		args := []string{"--namespace", namespace, "rollout", "status", "statefulset", name}

		cmd := exec.Command(kubectlBinary, args...)
		cmd.Env = os.Environ()
		o, _ := cmd.CombinedOutput()
		log.Printf("waitForStatefulset command: %s", cmd.String())
		output := string(o)
		log.Printf("waitForStatefulset output: %s", output)

		if strings.Contains(output, "is different from the running revision") {

			time.Sleep(1 * time.Second)
			continue
		}

		if strings.Contains(output, "partitioned roll out complete") || strings.Contains(output, "rolling update complete") {
			log.Println(output)
			return nil
		}

		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("%s didn't rollout after %d seconds", name, timeout)
}

func getContent(endpoint string, timeout int, upErrorChannel chan error) (string, error) {
	t := time.NewTicker(1 * time.Second)
	for i := 0; i < timeout; i++ {
		r, err := http.Get(endpoint)
		if err != nil {
			if !isUpRunning(upErrorChannel) {
				return "", fmt.Errorf("Up command is no longer running")
			}
			log.Printf("called %s, got %s, retrying", endpoint, err)
			<-t.C
			continue
		}

		defer r.Body.Close()
		if r.StatusCode != 200 {
			if !isUpRunning(upErrorChannel) {
				return "", fmt.Errorf("Up command is no longer running")
			}
			log.Printf("called %s, got status %d, retrying", endpoint, r.StatusCode)
			if !isUpRunning(upErrorChannel) {
				return "", fmt.Errorf("Up command is no longer running")
			}
			<-t.C
			continue
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			return "", err
		}
		if !isUpRunning(upErrorChannel) {
			return "", fmt.Errorf("Up command is no longer running")
		}

		return string(body), nil
	}

	return "", fmt.Errorf("service wasn't available")
}

func writeManifest(path, name string) error {
	oFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer oFile.Close()

	if err := manifestTemplate.Execute(oFile, deployment{Name: name}); err != nil {
		return err
	}

	return nil
}

func writeAutocreateManifest(path, name string) error {
	oFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer oFile.Close()

	if err := autocreateManifestTemplate.Execute(oFile, deployment{Name: name}); err != nil {
		return err
	}

	return nil
}

func createNamespace(ctx context.Context, oktetoPath, namespace string) error {
	okteto.CurrentStore = nil
	log.Printf("creating namespace %s", namespace)
	args := []string{"create", "namespace", namespace, "-l", "debug"}
	cmd := exec.Command(oktetoPath, args...)
	cmd.Env = os.Environ()
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", oktetoPath, strings.Join(args, " "), string(o))
	}

	log.Printf("create namespace output: \n%s\n", string(o))

	n := okteto.Context().Namespace
	if namespace != n {
		return fmt.Errorf("current namespace is %s, expected %s", n, namespace)
	}
	if err := updateKubeConfig(oktetoPath); err != nil {
		return err
	}
	return nil
}

func changeToNamespace(ctx context.Context, oktetoPath, namespace string) error {
	okteto.CurrentStore = nil
	log.Printf("changing to namespace %s", namespace)
	args := []string{"namespace", namespace, "-l", "debug"}
	cmd := exec.Command(oktetoPath, args...)
	cmd.Env = os.Environ()
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", oktetoPath, strings.Join(args, " "), string(o))
	}

	log.Printf("namespace output: \n%s\n", string(o))

	n := okteto.Context().Namespace
	if namespace != n {
		return fmt.Errorf("current namespace is %s, expected %s", n, namespace)
	}
	args = []string{"kubeconfig"}
	cmd = exec.Command(oktetoPath, args...)
	cmd.Env = os.Environ()
	o, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", oktetoPath, strings.Join(args, " "), string(o))
	}

	return nil
}

func deleteNamespace(ctx context.Context, oktetoPath, namespace string) error {
	log.Printf("okteto delete namespace %s", namespace)
	deleteCMD := exec.Command(oktetoPath, "delete", "namespace", namespace)
	deleteCMD.Env = os.Environ()
	o, err := deleteCMD.CombinedOutput()
	if err != nil {
		return fmt.Errorf("okteto delete namespace failed: %s - %s", string(o), err)
	}

	return nil
}

func testUpdateContent(content, contentPath string, timeout int, upErrorChannel chan error) error {
	start := time.Now()
	os.WriteFile(contentPath, []byte(content), 0644)

	if err := os.WriteFile(contentPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to update %s: %s", contentPath, err.Error())
	}

	log.Printf("getting updated content from %s\n", indexEndpoint)
	tick := time.NewTicker(1 * time.Second)
	gotUpdated := false
	counter := 0
	for i := 0; i < timeout; i++ {
		<-tick.C
		if !isUpRunning(upErrorChannel) {
			return fmt.Errorf("Up command is no longer running")
		}
		currentContent, err := getContent(indexEndpoint, 3, upErrorChannel)
		if err != nil {
			log.Printf("failed to get updated content: %s", err.Error())
			if strings.Contains(err.Error(), "Up command is no longer running") {
				return err
			}
			continue
		}

		if currentContent != content {
			counter++
			if counter%5 == 0 {
				log.Printf("expected updated index content to be %s, got %s\n", content, currentContent)
			}
			continue
		}

		log.Printf("got updated content after %v\n", time.Now().Sub(start))
		currentContent, err = getContent(varEndpoint, 3, upErrorChannel)
		if err != nil {
			return err
		}

		log.Println("got synchronized var content")

		if currentContent != "value2" {
			log.Printf("expected updated var content to be 'value2', got %s\n", currentContent)
			continue
		}

		gotUpdated = true
		break
	}

	if !gotUpdated {
		return fmt.Errorf("never got the updated content %s", content)
	}

	return nil
}

func isUpRunning(upErrorChannel chan error) bool {
	if upErrorChannel == nil {
		return true
	}
	select {
	case <-upErrorChannel:
		return false
	default:
		return true
	}
}

func testRemoteStignoreGenerated(ctx context.Context, namespace, name, manifestPath, oktetoPath string) error {
	cmd := exec.Command(oktetoPath, "exec", "-n", namespace, "-f", manifestPath, "--", "cat .stignore | grep '(?d)venv'")
	cmd.Env = os.Environ()
	log.Printf("exec command: %s", cmd.String())
	bytes, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("okteto exec failed: %v - %s", err, string(bytes))
		return fmt.Errorf("okteto exec failed: %v - %s", err, string(bytes))
	}
	output := string(bytes)
	if !strings.Contains(output, "venv") {
		return fmt.Errorf("okteto exec wrong output: %s", output)
	}
	return nil
}

func killLocalSyncthing() error {
	processes, err := ps.Processes()
	if err != nil {
		return fmt.Errorf("fail to list processes: %s", err.Error())
	}
	for _, p := range processes {
		if p.Executable() == "syncthing" {
			pr, err := os.FindProcess(p.Pid())
			if err != nil {
				log.Printf("fail to find process %d : %s", p.Pid(), err)
				continue
			}
			if err := pr.Kill(); err != nil {
				log.Printf("fail to kill process %d : %s", p.Pid(), err)
			}
		}
	}
	return nil
}

func destroyPod(ctx context.Context, name, namespace string) error {
	log.Printf("destroying pods of %s", name)
	c, _, err := K8sClient()
	if err != nil {
		return err
	}

	pods, err := c.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("app=%s", name)})
	if err != nil {
		return fmt.Errorf("failed to retrieve deployment %s pods: %s", name, err.Error())
	}

	for _, p := range pods.Items {
		log.Printf("destroying pod %s", p.Name)
		err := c.CoreV1().Pods(namespace).Delete(
			ctx,
			p.Name,
			metav1.DeleteOptions{GracePeriodSeconds: &zero},
		)
		if err != nil {
			return fmt.Errorf("error deleting pod %s: %s", p.Name, err.Error())
		}
	}

	return nil
}

func getPod(ctx context.Context, c *kubernetes.Clientset, name, namespace string) (*corev1.Pod, error) {
	log.Printf("getting pods of %s", name)

	pods, err := c.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("app=%s", name)})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve deployment %s pods: %s", name, err.Error())
	}
	if len(pods.Items) != 1 {
		return nil, fmt.Errorf("failed to retrieve deployment %s pod: %v", name, err)
	}
	return &pods.Items[0], nil

}

func down(ctx context.Context, namespace, name, manifestPath, oktetoPath string, isDeployment, skipComprobation bool) error {
	downCMD := exec.Command(oktetoPath, "down", "-n", namespace, "-f", manifestPath, "-v")
	downCMD.Env = os.Environ()
	o, err := downCMD.CombinedOutput()

	log.Printf("okteto down output:\n%s", string(o))
	if err != nil {
		m, _ := os.ReadFile(manifestPath)
		log.Printf("manifest: \n%s\n", string(m))
		return fmt.Errorf("okteto down failed: %s", err)
	}

	if !skipComprobation {
		if isDeployment {
			log.Println("waiting for the deployment to be restored")
			if err := waitForDeployment(ctx, namespace, name, 3, 120); err != nil {
				return err
			}
		} else {
			log.Println("waiting for the statefulset to be restored")
			if err := waitForStatefulset(ctx, namespace, name, 120); err != nil {
				return err
			}
		}
	}

	return nil
}

func downSvc(ctx context.Context, name, folder, oktetoPath string) error {
	downCMD := exec.Command(oktetoPath, "down", name, "-v")
	downCMD.Env = os.Environ()
	downCMD.Dir = folder
	o, err := downCMD.CombinedOutput()

	log.Printf("okteto down output:\n%s", string(o))
	if err != nil {
		return fmt.Errorf("okteto down failed: %s", err)
	}

	return nil
}

func up(ctx context.Context, wg *sync.WaitGroup, namespace, name, manifestPath, oktetoPath string, upErrorChannel chan error) (*os.Process, error) {
	var out bytes.Buffer
	cmd := exec.Command(oktetoPath, "up", "-n", namespace, "-f", manifestPath)
	cmd.Env = os.Environ()
	cmd.Stdout = &out
	cmd.Stderr = &out
	log.Printf("up command: %s", cmd.String())
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("okteto up failed to start: %s", err)
	}

	wg.Add(1)

	go func() {
		defer wg.Done()
		if err := cmd.Wait(); err != nil {
			if err != nil {
				log.Printf("okteto up exited: %s.\nOutput:\n%s", err, out.String())
				upErrorChannel <- fmt.Errorf("Okteto up exited before completion")
			}
		}
	}()

	return cmd.Process, waitForReady(namespace, name, upErrorChannel)
}

func waitForUpExit(wg *sync.WaitGroup) error {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(30 * time.Second):
		return fmt.Errorf("up didn't exit after 30 seconds")
	}
}

func waitForReady(namespace, name string, upErrorChannel chan error) error {
	log.Println("waiting for okteto up to be ready")

	state := path.Join(config.GetOktetoHome(), namespace, name, "okteto.state")

	t := time.NewTicker(1 * time.Second)
	for i := 0; i < 500; i++ {
		if !isUpRunning(upErrorChannel) {
			return fmt.Errorf("Okteto up exited before completion")
		}
		c, err := os.ReadFile(state)
		if err != nil {
			log.Printf("failed to read state file %s: %s", state, err)
			if !os.IsNotExist(err) {
				return err
			}

			<-t.C
			continue
		}

		if i%5 == 0 {
			log.Printf("okteto up is: %s", c)
		}

		if string(c) == "ready" {
			log.Printf("okteto up is: %s", c)
			return nil
		} else if string(c) == "failed" {
			return fmt.Errorf("development container failed")
		}

		<-t.C
	}

	return fmt.Errorf("development container was never ready")
}

func deploy(ctx context.Context, namespace, name, path string, isDeployment bool) error {
	log.Printf("deploying kubernetes manifest %s", path)
	cmd := exec.Command(kubectlBinary, "apply", "-n", namespace, "-f", path)
	cmd.Env = os.Environ()

	if o, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("kubectl apply failed: %s", string(o))
	}

	if isDeployment {
		if err := waitForDeployment(ctx, namespace, name, 1, 120); err != nil {
			return err
		}
	} else {
		if err := waitForStatefulset(ctx, namespace, name, 120); err != nil {
			return err
		}
	}

	return nil
}

func writeStatefulset(name, path string) error {
	dFile, err := os.Create(path)
	if err != nil {
		return err
	}

	if err := statefulsetTemplate.Execute(dFile, deployment{Name: name}); err != nil {
		return err
	}
	defer dFile.Close()
	return nil
}

func getStatefulset(ctx context.Context, ns, name string) (*appsv1.StatefulSet, error) {
	client, _, err := K8sClient()
	if err != nil {
		return nil, err
	}

	return client.AppsV1().StatefulSets(ns).Get(ctx, name, metav1.GetOptions{})
}

func getJob(ctx context.Context, ns, name string) (*batchv1.Job, error) {
	client, _, err := K8sClient()
	if err != nil {
		return nil, err
	}

	return client.BatchV1().Jobs(ns).Get(ctx, name, metav1.GetOptions{})
}

func getVolume(ctx context.Context, ns, name string) (*corev1.PersistentVolumeClaim, error) {
	client, _, err := K8sClient()
	if err != nil {
		return nil, err
	}
	return client.CoreV1().PersistentVolumeClaims(ns).Get(ctx, name, metav1.GetOptions{})
}

func compareStatefulset(ctx context.Context, statefulset *appsv1.StatefulSet) error {
	after, err := getStatefulset(ctx, statefulset.GetNamespace(), statefulset.GetName())
	if err != nil {
		return err
	}

	b, err := yaml.Marshal(statefulset.Spec)
	if err != nil {
		return err
	}

	a, err := yaml.Marshal(after.Spec)
	if err != nil {
		return err
	}

	if string(a) != string(b) {
		return fmt.Errorf("got:\n%s\nexpected:\n%s", string(a), string(b))
	}

	return nil
}

func compareDeployment(ctx context.Context, deployment *appsv1.Deployment) error {
	after, err := getDeployment(ctx, deployment.GetNamespace(), deployment.GetName())
	if err != nil {
		return err
	}

	b, err := yaml.Marshal(deployment.Spec)
	if err != nil {
		return err
	}

	a, err := yaml.Marshal(after.Spec)
	if err != nil {
		return err
	}

	if string(a) != string(b) {
		return fmt.Errorf("got:\n%s\nexpected:\n%s", string(a), string(b))
	}

	return nil
}

func checkIfUpFinished(ctx context.Context, pid int) error {
	var err error
	t := time.NewTicker(1 * time.Second)
	for i := 0; i < 30; i++ {
		var found ps.Process
		found, err = ps.FindProcess(pid)
		if err == nil && found == nil {
			return nil
		}

		if err != nil {
			err = fmt.Errorf("error when finding process: %s", err)
		} else if found != nil {
			err = fmt.Errorf("okteto up didn't exit after down")
		}

		<-t.C
	}

	return err
}

func getCurrentNamespace() string {
	return kubeconfig.CurrentNamespace(config.GetKubeconfigPath())
}
