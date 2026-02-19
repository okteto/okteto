//go:build integration
// +build integration

// Copyright 2023-2025 The Okteto Authors
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

package gateway

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	ps "github.com/mitchellh/go-ps"
	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	token           = ""
	kubectlBinary   = "kubectl"
	appsSubdomain   = ""
	ErrUpNotRunning = errors.New("Up command is no longer running")
)

const (
	timeout         = 300 * time.Second
	stignoreContent = `venv
.okteto
.kube`

	composeTemplate = `services:
  app:
    build: app
    command: echo value1 > /usr/src/app/var.html && python -m http.server 8080
    workdir: /usr/src/app
    ports:
      - 8080
    volumes:
    - .:/usr/src/app
  nginx:
    build: nginx
    volumes:
      - ./nginx/nginx.conf:/tmp/nginx.conf
    command: /bin/bash -c "envsubst < /tmp/nginx.conf > /etc/nginx/conf.d/default.conf && nginx -g 'daemon off;'"
    environment:
      - SERVER=app:8080
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
      start_period: 30s
`

	nginxConf = `server {
  listen 80;
  location / {
    proxy_pass http://$SERVER;
  }
}`

	nginxDockerfile = `FROM nginx
COPY ./nginx.conf /tmp/nginx.conf
`
)

func TestMain(m *testing.M) {
	if v := os.Getenv(model.OktetoAppsSubdomainEnvVar); v != "" {
		appsSubdomain = v
	}

	if runtime.GOOS == "windows" {
		kubectlBinary = "kubectl.exe"
	}
	if _, err := exec.LookPath(kubectlBinary); err != nil {
		log.Printf("kubectl is not in the path: %s", err)
		os.Exit(1)
	}
	token = integration.GetToken()

	exitCode := m.Run()
	os.Exit(exitCode)
}

func writeFile(filepath, content string) error {
	if err := os.WriteFile(filepath, []byte(content), 0600); err != nil {
		return err
	}
	return nil
}

func killLocalSyncthing(upPid int) error {
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
			if upPid == p.PPid() {
				if err := pr.Kill(); err != nil {
					log.Printf("fail to kill process %d : %s", p.Pid(), err)
				}
			}
		}
	}
	return nil
}

func waitUntilUpdatedContent(url, expectedContent string, timeout time.Duration, errorChan chan error) error {
	ticker := time.NewTicker(1 * time.Second)
	to := time.NewTicker(timeout)
	contentTimeout := 5 * time.Second
	retry := 0
	for {
		select {
		case <-errorChan:
			return fmt.Errorf("okteto up is no longer running")
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

func createAppDockerfile(dir string) error {
	if err := os.Mkdir(dir+"/app", 0700); err != nil {
		return err
	}

	appDockerfilePath := dir + "/app/Dockerfile"
	appDockerfileContent := []byte("FROM python:alpine")
	if err := os.WriteFile(appDockerfilePath, appDockerfileContent, 0600); err != nil {
		return err
	}
	return nil
}

func createNginxDir(dir string) error {
	if err := os.Mkdir(dir+"/nginx", 0700); err != nil {
		return err
	}

	nginxPath := dir + "/nginx/nginx.conf"
	nginxContent := []byte(nginxConf)
	if err := os.WriteFile(nginxPath, nginxContent, 0600); err != nil {
		return err
	}

	nginxDockerfilePath := dir + "/nginx/Dockerfile"
	nginxDockerfileContent := []byte(nginxDockerfile)
	if err := os.WriteFile(nginxDockerfilePath, nginxDockerfileContent, 0600); err != nil {
		return err
	}
	return nil
}

func compareDeployment(ctx context.Context, deployment *appsv1.Deployment, c kubernetes.Interface) error {
	after, err := integration.GetDeployment(ctx, deployment.GetNamespace(), deployment.GetName(), c)
	if err != nil {
		return err
	}

	if deployment.GetAnnotations()[model.DeployedByLabel] != after.GetAnnotations()[model.DeployedByLabel] {
		return fmt.Errorf("annotation 'dev.okteto.com/deployed-by' is different. before: '%s', after: '%s'", deployment.GetAnnotations()[model.DeployedByLabel], after.GetAnnotations()[model.DeployedByLabel])
	}
	if deployment.GetLabels()[model.DeployedByLabel] != after.GetLabels()[model.DeployedByLabel] {
		return fmt.Errorf("label 'dev.okteto.com/deployed-by' is different. before: '%s', after: '%s'", deployment.GetLabels()[model.DeployedByLabel], after.GetLabels()[model.DeployedByLabel])
	}

	if len(deployment.Spec.Template.Spec.Containers) != len(after.Spec.Template.Spec.Containers) {
		return fmt.Errorf("number of containers is different before: '%d', after: '%d'", len(deployment.Spec.Template.Spec.Containers), len(after.Spec.Template.Spec.Containers))
	}

	beforeContainer := deployment.Spec.Template.Spec.Containers[0]
	afterContainer := after.Spec.Template.Spec.Containers[0]
	if beforeContainer.Image != afterContainer.Image {
		return fmt.Errorf("image is different. before: '%s', after: '%s'", beforeContainer.Image, afterContainer.Image)
	}

	beforeCommand := strings.Join(beforeContainer.Command, " ")
	afterCommand := strings.Join(afterContainer.Command, " ")
	if beforeCommand != afterCommand {
		return fmt.Errorf("command is different. before: '%s', after: '%s'", beforeCommand, afterCommand)
	}

	beforeArgs := strings.Join(beforeContainer.Args, " ")
	afterArgs := strings.Join(afterContainer.Args, " ")
	if beforeArgs != afterArgs {
		return fmt.Errorf("args is different. before: '%s', after: '%s'", beforeArgs, afterArgs)
	}

	return nil
}

// createDeployComposeScenario creates a compose scenario for deploy tests
func createDeployComposeScenario(dir string) error {
	// Create nginx directory and files
	if err := os.Mkdir(dir+"/nginx", 0700); err != nil {
		return err
	}

	nginxPath := dir + "/nginx/nginx.conf"
	nginxContent := []byte(nginxConf)
	if err := os.WriteFile(nginxPath, nginxContent, 0600); err != nil {
		return err
	}

	nginxDockerfilePath := dir + "/nginx/Dockerfile"
	nginxDockerfileContent := []byte(nginxDockerfile)
	if err := os.WriteFile(nginxDockerfilePath, nginxDockerfileContent, 0600); err != nil {
		return err
	}

	// Create app directory and Dockerfile
	if err := createAppDockerfile(dir); err != nil {
		return err
	}

	// Create docker-compose.yml
	composePath := dir + "/docker-compose.yml"
	composeContent := []byte(composeTemplate)
	if err := os.WriteFile(composePath, composeContent, 0600); err != nil {
		return err
	}

	return nil
}
