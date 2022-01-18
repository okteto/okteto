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
	"testing"
	"text/template"
	"time"

	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
)

const (
	buildGitRepo   = "git@github.com:okteto/react-getting-started.git"
	buildGitFolder = "react-getting-started"
	buildManifest  = "okteto.yml"
)

func TestBuildCommand(t *testing.T) {

	ctx := context.Background()
	oktetoPath, err := getOktetoPath(ctx)
	if err != nil {
		t.Fatal(err)
	}

	testName := fmt.Sprintf("TestBuildCommand-%s", runtime.GOOS)
	testRef := strings.ToLower(fmt.Sprintf("%s-%d", testName, time.Now().Unix()))
	namespace := fmt.Sprintf("%s-%s", testRef, user)

	t.Run(testName, func(t *testing.T) {
		t.Logf("setup test %s", testName)

		// setup namespace for test
		startNamespace := getCurrentNamespace()
		defer changeToNamespace(ctx, oktetoPath, startNamespace)

		if err := createNamespace(ctx, oktetoPath, namespace); err != nil {
			t.Fatal(err)
		}
		defer deleteNamespace(ctx, oktetoPath, namespace)

		// clone repo
		if err := cloneGitRepo(ctx, buildGitRepo); err != nil {
			t.Fatal(err)
		}
		defer deleteGitRepo(ctx, buildGitFolder)

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}

		dir, err := os.MkdirTemp(cwd, testName)
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(dir)

		manifestPath := filepath.Join(dir, "okteto.yml")
		manifestContent := `build:
  react-getting-started:
    context: .
`
		if err := writeManifestV2(manifestPath, manifestContent, nil); err != nil {
			t.Fatal(err)
		}

		t.Logf("running okteto build command...")
		if err := oktetoBuild(ctx, oktetoPath, manifestPath); err != nil {
			t.Fatal(err)
		}

		t.Logf("checking at registry if the image has been built...")
		expectedImageTag := fmt.Sprintf("%s/%s/%s:dev", okteto.Context().Registry, namespace, buildGitFolder)
		if _, err := registry.GetImageTagWithDigest(expectedImageTag); err != nil {
			t.Fatal(err)
		}

	})
}

func oktetoBuild(ctx context.Context, oktetoPath, oktetoManifestPath string) error {
	cmd := exec.Command(oktetoPath, "build", "-f", oktetoManifestPath)
	cmd.Env = append(os.Environ(), "OKTETO_ENABLE_MANIFEST_V2=true")
	cmd.Dir = buildGitFolder
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("okteto build failed: %s - %s", string(o), err)
	}
	log.Printf("okteto build %s success", oktetoManifestPath)
	return nil
}

type manifestV2TemplateVars struct {
	Name string
}

func writeManifestV2(path, content string, vars *manifestV2TemplateVars) error {
	oFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer oFile.Close()

	template := template.Must(template.New("manifest_v2").Parse(content))
	if err := template.Execute(oFile, vars); err != nil {
		return err
	}

	return nil
}
