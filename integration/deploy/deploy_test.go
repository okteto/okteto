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

package deploy

import (
	"log"
	"os"
	"os/exec"
	"runtime"
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/okteto/okteto/pkg/model"
)

var (
	user          = ""
	kubectlBinary = "kubectl"
	appsSubdomain = "cloud.okteto.net"
)

const (
	k8sManifestName     = "k8s.yml"
	k8sManifestTemplate = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: e2etest
spec:
  replicas: 1
  selector:
    matchLabels:
      app: e2etest
  template:
    metadata:
      labels:
        app: e2etest
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
  name: e2etest
  annotations:
    dev.okteto.com/auto-ingress: "true"
spec:
  type: ClusterIP
  ports:
  - name: e2etest
    port: 8080
  selector:
    app: e2etest
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
	if _, err := exec.LookPath(kubectlBinary); err != nil {
		log.Printf("kubectl is not in the path: %s", err)
		os.Exit(1)
	}

	originalNamespace := integration.GetCurrentNamespace()

	exitCode := m.Run()

	oktetoPath, _ := integration.GetOktetoPath()
	commands.RunOktetoNamespace(oktetoPath, originalNamespace)
	os.Exit(exitCode)
}
