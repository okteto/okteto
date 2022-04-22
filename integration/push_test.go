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
	"runtime"
	"strings"
	"testing"
	"time"
)

const (
	pushGitRepo   = "git@github.com:okteto/react-getting-started.git"
	pushGitFolder = "react-getting-started"
	pushManifest  = "okteto.yml"
)

func TestPush(t *testing.T) {

	ctx := context.Background()
	oktetoPath, err := getOktetoPath(ctx)
	if err != nil {
		t.Fatal(err)
	}
	tName := fmt.Sprintf("TestPush-%s", runtime.GOOS)
	name := strings.ToLower(fmt.Sprintf("%s-%d", tName, time.Now().Unix()))
	namespace := fmt.Sprintf("%s-%s", name, user)
	t.Run(tName, func(t *testing.T) {
		log.Printf("running %s \n", tName)
		startNamespace := getCurrentNamespace()
		defer changeToNamespace(ctx, oktetoPath, startNamespace)
		if err := createNamespace(ctx, oktetoPath, namespace); err != nil {
			t.Fatal(err)
		}

		log.Printf("created namespace %s \n", namespace)

		if err := cloneGitRepo(ctx, pushGitRepo); err != nil {
			t.Fatal(err)
		}

		log.Printf("cloned repo %s \n", pushGitRepo)

		defer deleteGitRepo(ctx, pushGitFolder)

		if err := oktetoPush(ctx, oktetoPath, pushManifest); err != nil {
			t.Fatal(err)
		}

		log.Printf("pushed using %s \n", pushManifest)

		endpoint := fmt.Sprintf("https://react-getting-started-%s.%s", namespace, appsSubdomain)
		content, err := getContent(endpoint, 300, nil)
		if err != nil {
			t.Fatalf("failed to get app content: %s", err)
		}

		if !strings.Contains(content, "Web site created using create-react-app") {
			t.Fatalf("wrong app content: %s", content)
		}

		d, err := getDeployment(ctx, namespace, "react-getting-started")
		if err != nil {
			t.Fatalf("error getting 'react-getting-started' deployment: %s", err.Error())
		}

		imageName := fmt.Sprintf("registry.%s/%s/react-getting-started:okteto", appsSubdomain, namespace)
		if d.Spec.Template.Spec.Containers[0].Image != imageName {
			t.Fatalf("wrong image built for okteto push: expected '%s', got '%s'", imageName, d.Spec.Template.Spec.Containers[0].Image)
		}

		if err := deleteNamespace(ctx, oktetoPath, namespace); err != nil {
			log.Printf("failed to delete namespace %s: %s\n", namespace, err)
		}
	})
}

func oktetoPush(ctx context.Context, oktetoPath, oktetoManifestPath string) error {
	log.Printf("okteto push %s", oktetoManifestPath)
	cmd := exec.Command(oktetoPath, "push", "-d", "-f", oktetoManifestPath)
	cmd.Env = os.Environ()
	cmd.Dir = pushGitFolder
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("okteto push failed: %s - %s", string(o), err)
	}
	log.Printf("okteto push %s success", oktetoManifestPath)
	return nil
}
