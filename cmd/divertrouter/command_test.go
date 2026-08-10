// Copyright 2026 The Okteto Authors
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

package divertrouter

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/divert/router"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

const (
	healthPollTimeout  = 5 * time.Second
	healthPollInterval = 20 * time.Millisecond
)

func fakeEnv(vars map[string]string) func(string) string {
	return func(name string) string {
		return vars[name]
	}
}

// freePort returns a port that was free a moment ago. Good enough for a test, and the only
// option available: the config loader rejects port 0, so the server cannot be asked to
// pick an ephemeral port itself.
func freePort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())

	return port
}

func isHealthy(port int) bool {
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d%s", port, healthPath))
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// isServing reports whether a proxy port accepted a request at all. The upstream is
// unreachable in a unit test, so a bad gateway is a served request.
func isServing(port int) bool {
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/orders", port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return true
}

func TestDivertRouter_IsAHiddenCommand(t *testing.T) {
	cmd := DivertRouter(io.NewIOController())

	require.Equal(t, "divert-router", cmd.Use)
	require.True(t, cmd.Hidden)
	require.True(t, cmd.SilenceUsage)
}

// rootWithLogLevel mirrors how main.go declares the CLI-wide log level flag.
func rootWithLogLevel(t *testing.T) (*cobra.Command, *cobra.Command) {
	t.Helper()

	root := &cobra.Command{Use: "okteto"}
	var level string
	root.PersistentFlags().StringVarP(&level, logLevelFlag, "l", "warn", "")

	cmd := DivertRouter(io.NewIOController())
	root.AddCommand(cmd)

	return root, cmd
}

func TestShouldRaiseLogLevel_WhenNoLevelWasChosen(t *testing.T) {
	_, cmd := rootWithLogLevel(t)

	require.True(t, shouldRaiseLogLevel(cmd))
}

func TestShouldRaiseLogLevel_WhenALevelWasChosen(t *testing.T) {
	root, cmd := rootWithLogLevel(t)
	require.NoError(t, root.PersistentFlags().Set(logLevelFlag, "debug"))

	require.False(t, shouldRaiseLogLevel(cmd))
}

func TestShouldRaiseLogLevel_WhenTheCommandHasNoRootFlag(t *testing.T) {
	cmd := DivertRouter(io.NewIOController())

	require.True(t, shouldRaiseLogLevel(cmd))
}

func TestRun_RejectsAnIncompleteEnvironment(t *testing.T) {
	err := Run(context.Background(), fakeEnv(map[string]string{}), &testLogger{})

	require.ErrorContains(t, err, "invalid divert router configuration")
}

// runEnv is a complete router environment with the given ports.
func runEnv(healthPort int, listenPorts ...int) map[string]string {
	ports := make([]router.PortConfig, 0, len(listenPorts))
	for _, listen := range listenPorts {
		ports = append(ports, router.PortConfig{Listen: listen, Service: 80})
	}
	encoded, _ := json.Marshal(ports)

	return map[string]string{
		"SERVICE_NAME":     "api",
		"SHARED_NAMESPACE": "staging",
		"BASELINE_HOST":    "api-baseline.staging.svc.cluster.local",
		"PORTS":            string(encoded),
		"HEALTH_PORT":      fmt.Sprint(healthPort),
	}
}

func TestRun_RejectsAHealthPortThatShadowsTheProxyPort(t *testing.T) {
	err := Run(context.Background(), fakeEnv(runEnv(8080, 8080)), &testLogger{})

	require.ErrorContains(t, err, "readiness must not shadow a proxied path")
}

func TestRun_BecomesReadyAndShutsDownCleanly(t *testing.T) {
	healthPort := freePort(t)
	vars := runEnv(healthPort, freePort(t))
	vars["ROUTES"] = "alice:alice-dev"
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() { done <- Run(ctx, fakeEnv(vars), &testLogger{}) }()
	require.Eventually(t, func() bool { return isHealthy(healthPort) }, healthPollTimeout, healthPollInterval)

	cancel()

	require.NoError(t, <-done)
	require.False(t, isHealthy(healthPort))
}

// Every port of a multi-port Service has to be served, not just the first.
func TestRun_ServesEveryPortOfTheService(t *testing.T) {
	healthPort := freePort(t)
	first, second := freePort(t), freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() { done <- Run(ctx, fakeEnv(runEnv(healthPort, first, second)), &testLogger{}) }()
	require.Eventually(t, func() bool { return isHealthy(healthPort) }, healthPollTimeout, healthPollInterval)

	require.True(t, isServing(first))
	require.True(t, isServing(second))

	cancel()

	require.NoError(t, <-done)
}

func TestRun_ReadsRoutesFromAMountedDirectory(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "alice"), []byte("alice-dev"), 0o600))
	healthPort := freePort(t)
	vars := runEnv(healthPort, freePort(t))
	vars["ROUTES_DIR"] = dir
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() { done <- Run(ctx, fakeEnv(vars), &testLogger{}) }()
	require.Eventually(t, func() bool { return isHealthy(healthPort) }, healthPollTimeout, healthPollInterval)

	cancel()

	require.NoError(t, <-done)
}

func TestRouteTable_PrefersTheMountedDirectoryOverTheEnvironment(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "alice"), []byte("from-the-directory"), 0o600))
	cfg := &router.Config{Service: "api", Routes: "alice:from-the-environment", RoutesDir: dir}

	table := routeTable(context.Background(), cfg, &testLogger{})

	namespace, ok := table.Lookup("api", "alice")
	require.True(t, ok)
	require.Equal(t, "from-the-directory", namespace)
}

func TestRouteTable_FallsBackToTheEnvironment(t *testing.T) {
	cfg := &router.Config{Service: "api", Routes: "alice:from-the-environment"}

	table := routeTable(context.Background(), cfg, &testLogger{})

	namespace, ok := table.Lookup("api", "alice")
	require.True(t, ok)
	require.Equal(t, "from-the-environment", namespace)
}
