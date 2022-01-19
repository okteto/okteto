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
		gitRepo          = "git@github.com:okteto/react-getting-started.git"
		repoDir          = "react-getting-started"
		manifestFilename = "okteto.yml"
		manifestContent  = `build:
  react-getting-started:
    context: .`
	)

	var (
		testID           = strings.ToLower(fmt.Sprintf("TestBuildCommand-%s-%d", runtime.GOOS, time.Now().Unix()))
		testNamespace    = fmt.Sprintf("%s-%s", testID, user)
		originNamespace  = getCurrentNamespace()
		expectedImageTag = fmt.Sprintf("%s/%s/%s:dev", okteto.Context().Registry, testNamespace, repoDir)
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

	pathToManifestDir := filepath.Join(cwd, repoDir)
	if err := writeFile(pathToManifestDir, manifestFilename, manifestContent); err != nil {
		t.Fatal(err)
	}
	pathToManifestFile := filepath.Join(pathToManifestDir, manifestFilename)

	t.Cleanup(func() {
		changeToNamespace(ctx, oktetoPath, originNamespace)
		deleteNamespace(ctx, oktetoPath, testNamespace)
		deleteGitRepo(ctx, gitRepo)

	})

	t.Run("okteto build should build and push the image to registry", func(t *testing.T) {

		if _, err := registry.GetImageTagWithDigest(expectedImageTag); err == nil {
			t.Fatal("image is already at registry")
		}

		if err := runOktetoBuild(ctx, oktetoPath, pathToManifestFile, repoDir); err != nil {
			t.Fatal(err)
		}

		if _, err := registry.GetImageTagWithDigest(expectedImageTag); err != nil {
			t.Fatalf("image not pushed to registry: %s", err.Error())
		}

	})
}

func runOktetoBuild(ctx context.Context, oktetoPath, pathToManifestFile, repoDir string) error {
	cmd := exec.Command(oktetoPath, "build", "-f", pathToManifestFile)
	cmd.Env = append(os.Environ(), "OKTETO_ENABLE_MANIFEST_V2=true")
	cmd.Dir = repoDir
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("okteto build failed: %s - %s", string(o), err)
	}
	return nil
}
