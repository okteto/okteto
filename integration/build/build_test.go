//go:build integration
// +build integration

// Copyright 2023 The Okteto Authors
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
	"path/filepath"
	"runtime"
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/stretchr/testify/require"
)

var (
	user          = ""
	kubectlBinary = "kubectl"
	appsSubdomain = "cloud.okteto.net"
	token         = ""
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
	composeName    = "docker-compose.yml"
	composeContent = `
services:
  vols:
    build: .
    volumes:
    - Dockerfile:/root/Dockerfile
`
	composeWithVolumeMountsContent = `
services:
  nginx:
    image: nginx:latest
    volumes:
    - ./nginx/nginx.conf:/tmp/nginx.conf
`
	dockerfileName    = "Dockerfile"
	dockerfileContent = "FROM alpine"

	dockerfileContentSecrets = `FROM alpine
RUN --mount=type=secret,id=mysecret cat /run/secrets/mysecret
RUN --mount=type=secret,id=mysecret,dst=/other cat /other`

	secretFile             = "mysecret.txt"
	secretFileContent      = "secret"
	manifestContentSecrets = `
build:
    test:
      context: .
      dockerfile: Dockerfile
      secrets:
        mysecret: mysecret.txt
`

	nginxConf = `server {
  listen 80;
  location / {
    proxy_pass http://$FLASK_SERVER_ADDR;
  }
}`
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
	token = integration.GetToken()

	exitCode := m.Run()

	os.Exit(exitCode)
}

// TestBuildCommandV1 tests the following scenario:
// - building having a dockerfile
func TestBuildCommandV1(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, createDockerfile(dir))

	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	testNamespace := integration.GetTestNamespace("BuildV1", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts)

	expectedImage := fmt.Sprintf("%s/%s/test:okteto", okteto.GetContext().Registry, testNamespace)
	require.False(t, isImageBuilt(expectedImage))

	options := &commands.BuildOptions{
		Workdir:      dir,
		ManifestPath: filepath.Join(dir, dockerfileName),
		Tag:          "okteto.dev/test:okteto",
		Namespace:    testNamespace,
		Token:        token,
		OktetoHome:   dir,
	}
	require.NoError(t, commands.RunOktetoBuild(oktetoPath, options))
	require.True(t, isImageBuilt(expectedImage))
}

// TestBuildInferredDockerfile tests the following scenario:
// - building having a no manifest v2 in the folder but a Dockerfile
func TestBuildInferredDockerfile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, createDockerfile(dir))

	testNamespace := integration.GetTestNamespace("BuildInferredDockerfile", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}

	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts)

	expectedImage := fmt.Sprintf("%s/%s/test:okteto", okteto.GetContext().Registry, testNamespace)
	require.False(t, isImageBuilt(expectedImage))

	options := &commands.BuildOptions{
		Workdir:    dir,
		Tag:        "okteto.dev/test:okteto",
		Namespace:  "",
		Token:      token,
		OktetoHome: dir,
	}
	require.NoError(t, commands.RunOktetoBuild(oktetoPath, options))
	require.True(t, isImageBuilt(expectedImage))
}

// TestBuildCommandV2 tests the following scenario:
// - building having a manifest v2 with build section
func TestBuildCommandV2(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, createDockerfile(dir))
	require.NoError(t, createManifestV2(dir))

	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	testNamespace := integration.GetTestNamespace("BuildV2", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts)

	expectedAppImage := fmt.Sprintf("%s/%s/%s-app:okteto", okteto.GetContext().Registry, testNamespace, filepath.Base(dir))
	require.False(t, isImageBuilt(expectedAppImage))

	expectedApiImage := fmt.Sprintf("%s/%s/%s-api:okteto", okteto.GetContext().Registry, testNamespace, filepath.Base(dir))
	require.False(t, isImageBuilt(expectedApiImage))

	options := &commands.BuildOptions{
		Workdir:      dir,
		ManifestPath: filepath.Join(dir, manifestName),
		Namespace:    testNamespace,
		Token:        token,
		OktetoHome:   dir,
	}
	require.NoError(t, commands.RunOktetoBuild(oktetoPath, options))
	require.True(t, isImageBuilt(expectedAppImage))
	require.True(t, isImageBuilt(expectedApiImage))
}

// TestBuildCommandV2UsingDepot tests the following scenario:
// - use depot.dev as builder
// - building having a manifest v2 with build section
func TestBuildCommandV2UsingDepot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, createDockerfile(dir))
	require.NoError(t, createManifestV2(dir))

	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	testNamespace := integration.GetTestNamespace("BuildCommandV2UsingDepot", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts)

	expectedAppImage := fmt.Sprintf("%s/%s/%s-app:okteto", okteto.GetContext().Registry, testNamespace, filepath.Base(dir))
	require.False(t, isImageBuilt(expectedAppImage))

	expectedApiImage := fmt.Sprintf("%s/%s/%s-api:okteto", okteto.GetContext().Registry, testNamespace, filepath.Base(dir))
	require.False(t, isImageBuilt(expectedApiImage))

	options := &commands.BuildOptions{
		Workdir:      dir,
		ManifestPath: filepath.Join(dir, manifestName),
		Namespace:    testNamespace,
		Token:        token,
		OktetoHome:   dir,
	}

	cmd := commands.GetOktetoBuildCmd(oktetoPath, options)
	depotToken := os.Getenv("DEPOT_TOKEN")
	depotProjectId := os.Getenv("DEPOT_PROJECT_ID")

	cmd.Args = append(cmd.Args, "--log-level", "debug")
	if depotProjectId != "" && depotToken != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", build.DepotProjectEnvVar, depotProjectId))
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", build.DepotTokenEnvVar, depotToken))
	} else {
		// if the environment variables needed to run the test using depot.dev
		// are not available, skip it as the resulting e2e test would be similar
		// to TestBuildCommandV2.
		t.Skip()
	}

	require.NoError(t, commands.ExecOktetoBuildCmd(cmd))
	require.True(t, isImageBuilt(expectedAppImage))
	require.True(t, isImageBuilt(expectedApiImage))
}

// TestBuildCommandV2OnlyOneService tests the following scenario:
// - building having a manifest v2 with build section
// - okteto build with a service as argument
func TestBuildCommandV2OnlyOneService(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, createDockerfile(dir))
	require.NoError(t, createManifestV2(dir))

	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	testNamespace := integration.GetTestNamespace("PartialBuildV2", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts)

	expectedImage := fmt.Sprintf("%s/%s/%s-app:okteto", okteto.GetContext().Registry, testNamespace, filepath.Base(dir))
	require.False(t, isImageBuilt(expectedImage))

	options := &commands.BuildOptions{
		Workdir:      dir,
		ManifestPath: filepath.Join(dir, manifestName),
		SvcsToBuild:  []string{"app"},
		Namespace:    testNamespace,
		Token:        token,
		OktetoHome:   dir,
	}
	require.NoError(t, commands.RunOktetoBuild(oktetoPath, options))
	require.True(t, isImageBuilt(expectedImage))
}

// TestBuildCommandV2SpecifyingServices tests the following scenario:
// - building having a manifest v2 with build section
// - okteto build with several service as argument
func TestBuildCommandV2SpecifyingServices(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, createDockerfile(dir))
	require.NoError(t, createManifestV2(dir))

	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	testNamespace := integration.GetTestNamespace("CompleteBuildV2", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts)

	expectedAppImage := fmt.Sprintf("%s/%s/%s-app:okteto", okteto.GetContext().Registry, testNamespace, filepath.Base(dir))
	require.False(t, isImageBuilt(expectedAppImage))

	expectedApiImage := fmt.Sprintf("%s/%s/%s-api:okteto", okteto.GetContext().Registry, testNamespace, filepath.Base(dir))
	require.False(t, isImageBuilt(expectedApiImage))

	options := &commands.BuildOptions{
		Workdir:      dir,
		ManifestPath: filepath.Join(dir, manifestName),
		SvcsToBuild:  []string{"app", "api"},
		Namespace:    testNamespace,
		Token:        token,
		OktetoHome:   dir,
	}
	require.NoError(t, commands.RunOktetoBuild(oktetoPath, options))
	require.True(t, isImageBuilt(expectedAppImage))
	require.True(t, isImageBuilt(expectedApiImage))
}

// TestBuildCommandV2FromCompose tests the following scenario:
// - building having a compose file
func TestBuildCommandV2FromCompose(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, createDockerfile(dir))
	require.NoError(t, createCompose(dir, composeContent))

	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	testNamespace := integration.GetTestNamespace("BuildVolumesV2", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts)

	expectedBuildImage := fmt.Sprintf("%s/%s/%s-vols:okteto", okteto.GetContext().Registry, testNamespace, filepath.Base(dir))
	require.False(t, isImageBuilt(expectedBuildImage))

	options := &commands.BuildOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		Token:      token,
		OktetoHome: dir,
	}
	require.NoError(t, commands.RunOktetoBuild(oktetoPath, options))
	require.True(t, isImageBuilt(expectedBuildImage), "%s not found", expectedBuildImage)
}

// TestBuildCommandV2FromCompose tests the following scenario:
// - building having a compose file specifying an image and volume mounts
func TestBuildCommandV2WithVolumeMounts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, createDockerfile(dir))
	require.NoError(t, createCompose(dir, composeWithVolumeMountsContent))

	require.NoError(t, os.Mkdir(filepath.Join(dir, "nginx"), 0700))

	nginxPath := filepath.Join(dir, "nginx", "nginx.conf")
	nginxContent := []byte(nginxConf)
	require.NoError(t, os.WriteFile(nginxPath, nginxContent, 0600))

	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	testNamespace := integration.GetTestNamespace("BuildVolumeMountsV2", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts)

	expectedBuildImage := fmt.Sprintf("%s/%s/%s-nginx:okteto", okteto.GetContext().Registry, testNamespace, filepath.Base(dir))
	require.False(t, isImageBuilt(expectedBuildImage))

	options := &commands.BuildOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		Token:      token,
		OktetoHome: dir,
	}
	require.NoError(t, commands.RunOktetoBuild(oktetoPath, options))
	require.True(t, isImageBuilt(expectedBuildImage), "%s not found", expectedBuildImage)
}

// TestTestBuildCommandV2Secrets tests the following scenario:
// - build having an okteto manifest v2
// - inject secrets from manifest to build
func TestBuildCommandV2Secrets(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, createDockerfileWithSecretMount(dir))
	require.NoError(t, createManifestV2Secrets(dir))
	require.NoError(t, createSecretFile(dir))

	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	testNamespace := integration.GetTestNamespace("BuildSecretsV2", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts)

	expectedBuildImage := fmt.Sprintf("%s/%s/%s-test:okteto", okteto.GetContext().Registry, testNamespace, filepath.Base(dir))
	require.False(t, isImageBuilt(expectedBuildImage))

	options := &commands.BuildOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		Token:      token,
		OktetoHome: dir,
		NoCache:    true,
	}
	require.NoError(t, commands.RunOktetoBuild(oktetoPath, options))
	require.True(t, isImageBuilt(expectedBuildImage), "%s not found", expectedBuildImage)
}

func createDockerfile(dir string) error {
	dockerfilePath := filepath.Join(dir, dockerfileName)
	dockerfileContent := []byte(dockerfileContent)
	if err := os.WriteFile(dockerfilePath, dockerfileContent, 0600); err != nil {
		return err
	}
	return nil
}

func createDockerfileWithSecretMount(dir string) error {
	dockerfilePath := filepath.Join(dir, dockerfileName)
	dockerfileContent := []byte(dockerfileContentSecrets)
	if err := os.WriteFile(dockerfilePath, dockerfileContent, 0600); err != nil {
		return err
	}
	return nil
}

func createSecretFile(dir string) error {
	secretFilePath := filepath.Join(dir, secretFile)
	secretFileContent := []byte(secretFileContent)
	if err := os.WriteFile(secretFilePath, secretFileContent, 0600); err != nil {
		return err
	}
	return nil
}

func createManifestV2(dir string) error {
	manifestPath := filepath.Join(dir, manifestName)
	manifestBytes := []byte(manifestContent)
	if err := os.WriteFile(manifestPath, manifestBytes, 0600); err != nil {
		return err
	}
	return nil
}

func createManifestV2Secrets(dir string) error {
	manifestPath := filepath.Join(dir, manifestName)
	manifestBytes := []byte(manifestContentSecrets)
	if err := os.WriteFile(manifestPath, manifestBytes, 0600); err != nil {
		return err
	}
	return nil
}

func createCompose(dir, content string) error {
	manifestPath := filepath.Join(dir, composeName)
	manifestBytes := []byte(content)
	if err := os.WriteFile(manifestPath, manifestBytes, 0600); err != nil {
		return err
	}
	return nil
}

func isImageBuilt(image string) bool {
	reg := registry.NewOktetoRegistry(okteto.Config{})
	if _, err := reg.GetImageTagWithDigest(image); err == nil {
		return true
	}
	return false
}
