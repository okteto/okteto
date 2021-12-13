//go:build integration
// +build integration

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
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

var (
	moviesRepo    = "okteto/movies"
	moviesRepoURL = "git@github.com:okteto/movies.git"
	moviesFolder  = "movies"
)

func TestDeployDestroy(t *testing.T) {

	ctx := context.Background()
	oktetoPath, err := getOktetoPath(ctx)
	if err != nil {
		t.Fatal(err)
	}
	tName := fmt.Sprintf("TestDeploy-%s", runtime.GOOS)
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

		if err := cloneGitRepo(ctx, moviesRepoURL); err != nil {
			t.Fatal(err)
		}

		log.Printf("cloned repo %s \n", moviesRepo)

		defer deleteGitRepo(ctx, moviesFolder)

		data, err := ioutil.ReadFile(filepath.Join(moviesFolder, "okteto-pipeline.yml"))
		if err != nil {
			log.Fatal(err)
		}
		content := string(data)
		newContent := strings.ReplaceAll(content, "okteto ", fmt.Sprintf("%s ", oktetoPath))
		if err := os.WriteFile(filepath.Join(moviesFolder, "okteto-pipeline.yml"), []byte(newContent), 0600); err != nil {
			log.Fatal(err)
		}
		log.Printf("new content:\n %s \n", newContent)

		if err := oktetoDeploy(ctx, oktetoPath); err != nil {
			t.Fatal(err)
		}

		log.Printf("deployed \n")

		endpoint := fmt.Sprintf("https://movies-%s.%s", namespace, appsSubdomain)
		content, err = getContent(endpoint, 150, nil)
		if err != nil {
			t.Fatalf("failed to get app content: %s", err)
		}

		if !strings.Contains(content, "Movies") {
			t.Fatalf("wrong app content: %s", content)
		}

		if err := oktetoDestroy(ctx, oktetoPath); err != nil {
			t.Fatal(err)
		}

		if err := deleteNamespace(ctx, oktetoPath, namespace); err != nil {
			log.Printf("failed to delete namespace %s: %s\n", namespace, err)
		}
	})
}

func oktetoDeploy(ctx context.Context, oktetoManifestPath string) error {
	log.Printf("okteto deploy %s", oktetoManifestPath)
	cmd := exec.Command(oktetoManifestPath, "deploy")
	cmd.Env = append(os.Environ(), "OKTETO_GIT_COMMIT=dev")
	cmd.Dir = moviesFolder
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("okteto deploy failed: %s - %s", string(o), err)
	}
	log.Printf("okteto deploy success")
	return nil
}

func oktetoDestroy(ctx context.Context, oktetoManifestPath string) error {
	log.Printf("okteto destroy %s", oktetoManifestPath)
	cmd := exec.Command(oktetoManifestPath, "destroy")
	cmd.Env = os.Environ()
	cmd.Dir = moviesFolder
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("okteto destroy failed: %s - %s", string(o), err)
	}
	log.Printf("okteto destroy success")
	return nil
}
