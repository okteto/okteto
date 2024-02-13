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

package up

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/stretchr/testify/require"
)

const (
	globalForwardManifest = `forward:
- localPort: 8093
  remotePort: 8080
  name: svc3
- 8091:svc1:8080
- 8092:svc2:8080
deploy:
  compose: docker-compose.yml
dev:
  svc1:
    image: python:alpine
    command:
    - sh
    - -c
    - echo svc1 > index.html && python -m http.server 8080
    sync:
    - .:/app
  svc2:
    image: python:alpine
    command:
    - sh
    - -c
    - echo svc2 > index.html && python -m http.server 8080
    sync:
    - .:/app
  svc3:
    image: python:alpine
    command:
    - sh
    - -c
    - echo svc3 > index.html && python -m http.server 8080
    sync:
    - .:/app
`
	globalForwardCompose = `services:
  svc1:
    image: python:alpine
    ports:
    - 8080
    command:
    - sh
    - -c
    - echo svc1 > index.html && python -m http.server 8080
  svc2:
    image: python:alpine
    ports:
    - 8080
    command:
    - sh
    - -c
    - echo svc2 > index.html && python -m http.server 8080
  svc3:
    image: python:alpine
    ports:
    - 8080
    command:
    - sh
    - -c
    - echo svc3 > index.html && python -m http.server 8080
`
)

func TestUpGlobalForwarding(t *testing.T) {
	t.Parallel()

	// Prepare environment
	dir := t.TempDir()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	testNamespace := integration.GetTestNamespace("GlobalFwd", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts)
	require.NoError(t, commands.RunOktetoKubeconfig(oktetoPath, dir))

	require.NoError(t, writeFile(filepath.Join(dir, "docker-compose.yml"), globalForwardCompose))
	require.NoError(t, writeFile(filepath.Join(dir, "okteto.yml"), globalForwardManifest))
	require.NoError(t, writeFile(filepath.Join(dir, ".stignore"), stignoreContent))

	up1Options := &commands.UpOptions{
		Name:       "svc1",
		Namespace:  testNamespace,
		Workdir:    dir,
		OktetoHome: dir,
		Token:      token,
		Service:    "svc1",
		Deploy:     true,
	}
	up1Result, err := commands.RunOktetoUp(oktetoPath, up1Options)
	require.NoError(t, err)

	svc1LocalEndpoint := "http://localhost:8091/index.html"
	svc2LocalEndpoint := "http://localhost:8092/index.html"
	svc3LocalEndpoint := "http://localhost:8093/index.html"

	// Test that all endpoints are up and working
	require.Equal(t, integration.GetContentFromURL(svc1LocalEndpoint, timeout), "svc1")
	require.Equal(t, integration.GetContentFromURL(svc2LocalEndpoint, timeout), "svc2")
	require.Equal(t, integration.GetContentFromURL(svc3LocalEndpoint, timeout), "svc3")

	// Test that running another up on another service doesn't break the previous one
	up2Options := &commands.UpOptions{
		Name:       "svc2",
		Namespace:  testNamespace,
		Workdir:    dir,
		OktetoHome: dir,
		Token:      token,
		Service:    "svc2",
	}
	up2Result, err := commands.RunOktetoUp(oktetoPath, up2Options)
	require.NoError(t, err)

	require.False(t, commands.HasUpCommandFinished(up1Result.Pid.Pid), "Up1 output: %s", up1Result.Output)

	// Test that all endpoints continue being up and working
	require.Equal(t, integration.GetContentFromURL(svc1LocalEndpoint, timeout), "svc1")
	require.Equal(t, integration.GetContentFromURL(svc2LocalEndpoint, timeout), "svc2")
	require.Equal(t, integration.GetContentFromURL(svc3LocalEndpoint, timeout), "svc3")

	// Test okteto down command
	down1Opts := &commands.DownOptions{
		Namespace: testNamespace,
		Workdir:   dir,
		Service:   "svc1",
		Token:     token,
	}
	require.NoError(t, commands.RunOktetoDown(oktetoPath, down1Opts))
	require.True(t, commands.HasUpCommandFinished(up1Result.Pid.Pid))

	// Test that all endpoints are up and working
	require.Equal(t, integration.GetContentFromURL(svc1LocalEndpoint, timeout), "svc1")
	require.Equal(t, integration.GetContentFromURL(svc2LocalEndpoint, timeout), "svc2")
	require.Equal(t, integration.GetContentFromURL(svc3LocalEndpoint, timeout), "svc3")

	// Test okteto down command
	down2Opts := &commands.DownOptions{
		Namespace: testNamespace,
		Workdir:   dir,
		Service:   "svc2",
		Token:     token,
	}
	require.NoError(t, commands.RunOktetoDown(oktetoPath, down2Opts))
	require.True(t, commands.HasUpCommandFinished(up2Result.Pid.Pid))

	// Test that all endpoints are down
	require.Equal(t, integration.GetContentFromURL(svc1LocalEndpoint, 20*time.Second), "")
	require.Equal(t, integration.GetContentFromURL(svc2LocalEndpoint, 5*time.Second), "")
	require.Equal(t, integration.GetContentFromURL(svc3LocalEndpoint, 5*time.Second), "")
}
