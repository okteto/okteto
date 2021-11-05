//go:build common
// +build common

// Copyright 2021 The Okteto Authors
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
	"path/filepath"
	"runtime"
	"testing"
	"text/template"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type deployment struct {
	Name string
}

var (
	user          = ""
	kubectlBinary = "kubectl"
	appsSubdomain = "cloud.okteto.net"
)

func TestMain(m *testing.M) {
	if u, ok := os.LookupEnv("OKTETO_USER"); !ok {
		log.Println("OKTETO_USER is not defined")
		os.Exit(1)
	} else {
		user = u
	}

	if v := os.Getenv("OKTETO_APPS_SUBDOMAIN"); v != "" {
		appsSubdomain = v
	}

	if runtime.GOOS == "windows" {
		kubectlBinary = "kubectl.exe"
	}

	os.Exit(m.Run())
}

func cloneGitRepo(ctx context.Context, name string) error {
	log.Printf("cloning git repo %s", name)
	cmd := exec.Command("git", "clone", name)
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cloning git repo %s failed: %s - %s", name, string(o), err)
	}
	log.Printf("clone git repo %s success", name)
	return nil
}

func deleteGitRepo(ctx context.Context, path string) error {
	log.Printf("delete git repo %s", path)
	err := os.RemoveAll(path)
	if err != nil {
		return fmt.Errorf("delete git repo %s failed: %w", path, err)
	}

	log.Printf("deleted git repo %s", path)
	return nil
}

func getOktetoPath(ctx context.Context) (string, error) {
	oktetoPath, ok := os.LookupEnv("OKTETO_PATH")
	if !ok {
		oktetoPath = "/usr/local/bin/okteto"
	}

	log.Printf("using %s", oktetoPath)

	var err error
	oktetoPath, err = filepath.Abs(oktetoPath)
	if err != nil {
		return "", err
	}

	cmd := exec.Command(oktetoPath, "version")
	cmd.Env = os.Environ()

	o, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("okteto version failed: %s - %s", string(o), err)
	}

	log.Println(string(o))
	return oktetoPath, nil
}

func getDeployment(ctx context.Context, ns, name string) (*appsv1.Deployment, error) {
	client, _, err := K8sClient()
	if err != nil {
		return nil, err
	}

	return client.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
}

func writeDeployment(template *template.Template, name, path string) error {
	dFile, err := os.Create(path)
	if err != nil {
		return err
	}

	if err := template.Execute(dFile, deployment{Name: name}); err != nil {
		return err
	}
	defer dFile.Close()
	return nil
}
