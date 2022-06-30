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
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	appsv1 "k8s.io/api/apps/v1"
)

func TestAutoWake(t *testing.T) {
	tName := fmt.Sprintf("Test-%s", runtime.GOOS)
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

	dPath := filepath.Join(dir, "deployment.yaml")
	if err := writeDeployment(deploymentTemplate, name, dPath); err != nil {
		t.Fatal(err)
	}

	sfsPath := filepath.Join(dir, "sfs.yaml")
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

	if err := deploy(ctx, namespace, name, dPath, true); err != nil {
		t.Fatal(err)
	}

	if err := deploy(ctx, namespace, name, sfsPath, true); err != nil {
		t.Fatal(err)
	}

	client, err := okteto.NewOktetoClient()
	if err != nil {
		t.Fatal(err)
	}

	endpoint := fmt.Sprintf("https://%s-%s.%s", name, namespace, appsSubdomain)
	content, err := getContent(endpoint, 300, nil)
	if err != nil {
		t.Fatalf("failed to get content: %s", err)
	}
	if content == "" {
		t.Fatalf("failed to get content")
	}

	if err := client.Namespaces().SleepNamespace(ctx, namespace); err != nil {
		t.Fatal(err)
	}

	time.Sleep(5 * time.Second)
	if err := checkIfSleeping(ctx, name, namespace, 300); err != nil {
		t.Fatal(err)
	}

	content, err = getContent(endpoint, 300, nil)
	if err != nil {
		t.Fatalf("failed to get content: %s", err)
	}
	if content == "" {
		t.Fatalf("failed to get content")
	}

	if err := checkIfAwake(ctx, name, namespace, false, 300); err != nil {
		t.Fatal(err)
	}

	if err := client.Namespaces().SleepNamespace(ctx, namespace); err != nil {
		t.Fatal(err)
	}

	if err := checkIfSleeping(ctx, name, namespace, 300); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	upErrorChannel := make(chan error, 1)
	p, err := up(ctx, &wg, namespace, name, manifestPath, oktetoPath, upErrorChannel)
	if err != nil {
		t.Fatal(err)
	}

	if err := checkIfAwake(ctx, name, namespace, true, 300); err != nil {
		t.Fatal(err)
	}

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

func checkIfSleeping(ctx context.Context, name, namespace string, timeout int) error {
	t := time.NewTicker(3 * time.Second)
	for i := 0; i < timeout; i++ {
		<-t.C
		d, err := getDeployment(ctx, namespace, name)
		if err != nil {
			log.Printf("error getting deployment: %s", err.Error())
			continue
		}

		if _, ok := d.Annotations[model.StateBeforeSleepingAnnontation]; !ok {
			log.Printf("error deployment: not sleeping")
			continue
		}
		sfs, err := getStatefulset(ctx, namespace, name)
		if err != nil {
			log.Printf("error getting sfs: %s", err.Error())
			continue
		}
		if _, ok := sfs.Annotations[model.StateBeforeSleepingAnnontation]; !ok {
			log.Printf("error deployment: not sleeping")
			continue
		}
		return nil
	}

	return fmt.Errorf("Resources not sleeping")
}
func checkIfAwake(ctx context.Context, name, namespace string, isDevMode bool, timeout int) error {
	t := time.NewTicker(1 * time.Second)
	for i := 0; i < timeout; i++ {
		<-t.C
		var d *appsv1.Deployment
		var err error
		if isDevMode {
			d, err = getDeployment(ctx, namespace, model.DevCloneName(name))
		} else {
			d, err = getDeployment(ctx, namespace, name)
		}

		if err != nil {
			log.Printf("error getting deployment: %s", err.Error())
			continue
		}

		if *d.Spec.Replicas == 0 {
			log.Printf("error dep: Not awake")
			continue
		}

		sfs, err := getStatefulset(ctx, namespace, name)
		if err != nil {
			log.Printf("error getting sfs: %s", err.Error())
			continue
		}
		if *sfs.Spec.Replicas == 0 {
			log.Printf("error sfs: Not awake")
			continue
		}
		return nil
	}

	return fmt.Errorf("Resources are not awake")
}
