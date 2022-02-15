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

func TestBuildCommand(t *testing.T) {

	ctx := context.Background()
	oktetoPath, err := getOktetoPath(ctx)
	if err != nil {
		t.Fatal(err)
	}
	const (
		gitRepo          = "git@github.com:okteto/go-getting-started.git"
		repoDir          = "go-getting-started"
		manifestFilename = "okteto.yml"
		manifestContent  = `
build:
  app:
    context: .`
	)

	var (
		testID           = strings.ToLower(fmt.Sprintf("TestBuildCommand-%s-%d", runtime.GOOS, time.Now().Unix()))
		testNamespace    = fmt.Sprintf("%s-%s", testID, user)
		originNamespace  = getCurrentNamespace()
		expectedImageTag = fmt.Sprintf("%s/%s/%s-app:okteto", okteto.Context().Registry, testNamespace, repoDir)
	)

	if err := cloneGitRepo(ctx, gitRepo); err != nil {
		t.Fatal(err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	pathToManifestDir := filepath.Join(cwd, repoDir)
	if err := writeFile(pathToManifestDir, manifestFilename, manifestContent); err != nil {
		t.Fatal(err)
	}
	pathToManifestFile := filepath.Join(pathToManifestDir, manifestFilename)

	if err := createNamespace(ctx, oktetoPath, testNamespace); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		changeToNamespace(ctx, oktetoPath, originNamespace)
		deleteNamespace(ctx, oktetoPath, testNamespace)
		deleteGitRepo(ctx, repoDir)

	})

	t.Run("okteto build should build and push the image to registry", func(t *testing.T) {

		if _, err := registry.GetImageTagWithDigest(expectedImageTag); err == nil {
			t.Fatal("image is already at registry")
		}

		output, err := runOktetoBuild(ctx, oktetoPath, pathToManifestFile, repoDir)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := registry.GetImageTagWithDigest(expectedImageTag); err != nil {
			t.Fatalf("image not pushed to registry: %v \nbuild output: %s", err, output)
		}

	})
}

func runOktetoBuild(ctx context.Context, oktetoPath, pathToManifestFile, repoDir string) (string, error) {
	cmd := exec.Command(oktetoPath, "build", "-f", pathToManifestFile, "-l", "debug")
	cmd.Dir = repoDir
	o, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("okteto build failed: \nerror: %s \noutput: %s", err.Error(), string(o))
	}
	return string(o), nil
}
