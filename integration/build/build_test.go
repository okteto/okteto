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

package build

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/stretchr/testify/assert"
)

var (
	user          = ""
	kubectlBinary = "kubectl"
	appsSubdomain = "cloud.okteto.net"
)

const (
	manifestName    = "okteto.yml"
	manifestContent = `
build:
  app:
    context: .
  api:
    context: .
    dockerfile: Dockerfile
`
	dockerfileName    = "Dockerfile"
	dockerfileContent = "FROM alpine"
)

type buildOptions struct {
	wd           string
	manifestPath string
	svcsToBuild  []string
	tag          string
}

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

	originalNamespace := integration.GetCurrentNamespace()

	exitCode := m.Run()

	oktetoPath, _ := integration.GetOktetoPath()
	integration.RunOktetoNamespace(oktetoPath, originalNamespace)
	os.Exit(exitCode)
}

func TestBuildCommandV1(t *testing.T) {
	dir := t.TempDir()
	assert.NoError(t, createDockerfile(dir))

	oktetoPath, err := integration.GetOktetoPath()
	assert.NoError(t, err)

	testNamespace := integration.GetTestNamespace("TestBuild", user)
	assert.NoError(t, integration.RunOktetoCreateNamespace(oktetoPath, testNamespace))
	defer integration.RunOktetoDeleteNamespace(oktetoPath, testNamespace)

	expectedImage := fmt.Sprintf("%s/%s/test:okteto", okteto.Context().Registry, testNamespace)
	assert.False(t, imageIsAlreadyBuild(expectedImage))

	options := &buildOptions{
		wd:           dir,
		manifestPath: filepath.Join(dir, dockerfileName),
		tag:          "okteto.dev/test:okteto",
	}
	assert.NoError(t, runOktetoBuild(oktetoPath, options))
	assert.True(t, imageIsAlreadyBuild(expectedImage))
}

func TestBuildCommandV2(t *testing.T) {
	dir := t.TempDir()
	assert.NoError(t, createDockerfile(dir))
	assert.NoError(t, createManifestV2(dir))

	oktetoPath, err := integration.GetOktetoPath()
	assert.NoError(t, err)

	testNamespace := integration.GetTestNamespace("TestBuild", user)
	assert.NoError(t, integration.RunOktetoCreateNamespace(oktetoPath, testNamespace))
	defer integration.RunOktetoDeleteNamespace(oktetoPath, testNamespace)

	expectedAppImage := fmt.Sprintf("%s/%s/%s-app:okteto", okteto.Context().Registry, testNamespace, filepath.Base(dir))
	assert.False(t, imageIsAlreadyBuild(expectedAppImage))

	expectedApiImage := fmt.Sprintf("%s/%s/%s-api:okteto", okteto.Context().Registry, testNamespace, filepath.Base(dir))
	assert.False(t, imageIsAlreadyBuild(expectedApiImage))

	options := &buildOptions{
		wd:           dir,
		manifestPath: filepath.Join(dir, manifestName),
	}
	assert.NoError(t, runOktetoBuild(oktetoPath, options))
	assert.True(t, imageIsAlreadyBuild(expectedAppImage))
	assert.True(t, imageIsAlreadyBuild(expectedApiImage))
}

func TestBuildCommandV2OnlyOneService(t *testing.T) {
	dir := t.TempDir()
	assert.NoError(t, createDockerfile(dir))
	assert.NoError(t, createManifestV2(dir))

	oktetoPath, err := integration.GetOktetoPath()
	assert.NoError(t, err)

	testNamespace := integration.GetTestNamespace("TestBuild", user)
	assert.NoError(t, integration.RunOktetoCreateNamespace(oktetoPath, testNamespace))
	defer integration.RunOktetoDeleteNamespace(oktetoPath, testNamespace)

	expectedImage := fmt.Sprintf("%s/%s/%s-app:okteto", okteto.Context().Registry, testNamespace, filepath.Base(dir))
	assert.False(t, imageIsAlreadyBuild(expectedImage))

	options := &buildOptions{
		wd:           dir,
		manifestPath: filepath.Join(dir, manifestName),
		svcsToBuild:  []string{"app"},
	}
	assert.NoError(t, runOktetoBuild(oktetoPath, options))
	assert.True(t, imageIsAlreadyBuild(expectedImage))
}

func TestBuildCommandV2SpecifyingServices(t *testing.T) {
	dir := t.TempDir()
	assert.NoError(t, createDockerfile(dir))
	assert.NoError(t, createManifestV2(dir))

	oktetoPath, err := integration.GetOktetoPath()
	assert.NoError(t, err)

	testNamespace := integration.GetTestNamespace("TestBuild", user)
	assert.NoError(t, integration.RunOktetoCreateNamespace(oktetoPath, testNamespace))
	defer integration.RunOktetoDeleteNamespace(oktetoPath, testNamespace)

	expectedAppImage := fmt.Sprintf("%s/%s/%s-app:okteto", okteto.Context().Registry, testNamespace, filepath.Base(dir))
	assert.False(t, imageIsAlreadyBuild(expectedAppImage))

	expectedApiImage := fmt.Sprintf("%s/%s/%s-api:okteto", okteto.Context().Registry, testNamespace, filepath.Base(dir))
	assert.False(t, imageIsAlreadyBuild(expectedApiImage))

	options := &buildOptions{
		wd:           dir,
		manifestPath: filepath.Join(dir, manifestName),
		svcsToBuild:  []string{"app", "api"},
	}
	assert.NoError(t, runOktetoBuild(oktetoPath, options))
	assert.True(t, imageIsAlreadyBuild(expectedAppImage))
	assert.True(t, imageIsAlreadyBuild(expectedApiImage))
}

func createDockerfile(dir string) error {
	dockerfilePath := filepath.Join(dir, dockerfileName)
	dockerfileContent := []byte(dockerfileContent)
	if err := os.WriteFile(dockerfilePath, dockerfileContent, 0644); err != nil {
		return err
	}
	return nil
}

func createManifestV2(dir string) error {
	manifestPath := filepath.Join(dir, manifestName)
	manifestBytes := []byte(manifestContent)
	if err := os.WriteFile(manifestPath, manifestBytes, 0644); err != nil {
		return err
	}
	return nil
}

func imageIsAlreadyBuild(image string) bool {
	reg := registry.NewOktetoRegistry()
	if _, err := reg.GetImageTagWithDigest(image); err == nil {
		return true
	}
	return false
}

func runOktetoBuild(oktetoPath string, buildOptions *buildOptions) error {
	cmd := exec.Command(oktetoPath)
	cmd.Args = append(cmd.Args, "build")
	if buildOptions.manifestPath != "" {
		cmd.Args = append(cmd.Args, "-f", buildOptions.manifestPath)
	}
	if buildOptions.tag != "" {
		cmd.Args = append(cmd.Args, "-t", buildOptions.tag)
	}
	if buildOptions.wd != "" {
		cmd.Dir = buildOptions.wd
	}
	if len(buildOptions.svcsToBuild) > 0 {
		cmd.Args = append(cmd.Args, buildOptions.svcsToBuild...)
	}

	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("okteto build failed: \nerror: %s \noutput:\n %s", err.Error(), string(o))
	}
	return nil
}
