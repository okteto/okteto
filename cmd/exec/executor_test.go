// Copyright 2024 The Okteto Authors
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

package exec

import (
	"context"
	"testing"

	"github.com/okteto/okteto/internal/sshtransport"
	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/tools/clientcmd/api"
)

func TestSSHExecutorUsesStoredEndpoint(t *testing.T) {
	t.Parallel()

	dev := &model.Dev{Name: "dev", Interface: "0.0.0.0"}
	var gotName, gotHost string
	var gotPort int
	var gotCommand []string
	executor := &sshExecutor{
		dev: dev,
		getStoredEndpoint: func(name string) (string, int, error) {
			gotName = name
			return sshtransport.LoopbackHost, 23456, nil
		},
		runRemoteCommand: func(_ context.Context, host string, port int, command []string) error {
			gotHost = host
			gotPort = port
			gotCommand = append([]string(nil), command...)
			return nil
		},
	}

	err := executor.execute(context.Background(), []string{"echo", "safe"})
	assert.NoError(t, err)
	assert.Equal(t, "dev", gotName)
	assert.Equal(t, sshtransport.LoopbackHost, gotHost)
	assert.Equal(t, 23456, gotPort)
	assert.Equal(t, []string{"echo", "safe"}, gotCommand)
	assert.Equal(t, "0.0.0.0", dev.Interface, "user-facing bind interface changed")
}

func TestSSHExecutorRejectsUnsafeInjectedEndpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		iface      string
		storedHost string
		port       int
		local      bool
		localErr   error
	}{
		{name: "hostname interface", iface: "ssh.example.com", storedHost: "ssh.example.com", port: 2222},
		{name: "empty interface", iface: "", storedHost: "127.0.0.1", port: 2222},
		{name: "unassigned IPv4", iface: "192.0.2.10", storedHost: "192.0.2.10", port: 2222},
		{name: "unassigned IPv6", iface: "2001:db8::10", storedHost: "2001:db8::10", port: 2222},
		{name: "interface lookup error", iface: "192.0.2.10", storedHost: "192.0.2.10", port: 2222, localErr: assert.AnError},
		{name: "zero port", iface: model.Localhost, storedHost: "127.0.0.1", port: 0},
		{name: "negative port", iface: model.Localhost, storedHost: "127.0.0.1", port: -1},
		{name: "large port", iface: model.Localhost, storedHost: "127.0.0.1", port: 65536},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			called := false
			executor := &sshExecutor{
				dev: &model.Dev{Name: "dev", Interface: tt.iface},
				getStoredEndpoint: func(string) (string, int, error) {
					return tt.storedHost, tt.port, nil
				},
				isLocalAddress: func(string) (bool, error) {
					return tt.local, tt.localErr
				},
				runRemoteCommand: func(context.Context, string, int, []string) error {
					called = true
					return nil
				},
			}

			err := executor.execute(context.Background(), []string{"whoami"})
			assert.Error(t, err)
			assert.False(t, called, "unsafe endpoint reached the SSH client")
		})
	}
}

func TestSSHExecutorAllowsAssignedConcreteInterface(t *testing.T) {
	t.Parallel()

	var gotHost string
	executor := &sshExecutor{
		dev: &model.Dev{Name: "dev", Interface: "192.0.2.10"},
		getStoredEndpoint: func(string) (string, int, error) {
			return "192.0.2.10", 23456, nil
		},
		isLocalAddress: func(host string) (bool, error) {
			assert.Equal(t, "192.0.2.10", host)
			return true, nil
		},
		runRemoteCommand: func(_ context.Context, host string, _ int, _ []string) error {
			gotHost = host
			return nil
		},
	}

	assert.NoError(t, executor.execute(context.Background(), []string{"whoami"}))
	assert.Equal(t, "192.0.2.10", gotHost)
}

func TestSSHExecutorRejectsStoredHostMismatch(t *testing.T) {
	t.Parallel()

	called := false
	executor := &sshExecutor{
		dev: &model.Dev{Name: "dev", Interface: "0.0.0.0"},
		getStoredEndpoint: func(string) (string, int, error) {
			return "192.0.2.10", 23456, nil
		},
		runRemoteCommand: func(context.Context, string, int, []string) error {
			called = true
			return nil
		},
	}

	assert.Error(t, executor.execute(context.Background(), []string{"whoami"}))
	assert.False(t, called)
}

func TestSSHExecutorAcceptsLegacyStoredBindHost(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		iface    string
		wantDial string
	}{
		{iface: model.Localhost, wantDial: "127.0.0.1"},
		{iface: "0.0.0.0", wantDial: "127.0.0.1"},
		{iface: "::", wantDial: "::1"},
	} {
		t.Run(tt.iface, func(t *testing.T) {
			t.Parallel()
			var gotHost string
			executor := &sshExecutor{
				dev: &model.Dev{Name: "dev", Interface: tt.iface},
				getStoredEndpoint: func(string) (string, int, error) {
					return tt.iface, 23456, nil
				},
				runRemoteCommand: func(_ context.Context, host string, _ int, _ []string) error {
					gotHost = host
					return nil
				},
			}

			assert.NoError(t, executor.execute(context.Background(), []string{"whoami"}))
			assert.Equal(t, tt.wantDial, gotHost)
		})
	}
}

func TestExec_getExecutor(t *testing.T) {
	namespace := "test"

	e := executorProvider{
		ioCtrl:            io.NewIOController(),
		k8sClientProvider: test.NewFakeK8sProvider(),
	}

	dev := &model.Dev{} // Create a sample dev object
	podName := "test-pod"

	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Cfg: api.NewConfig(),
			},
		},
		CurrentContext: "test",
	}
	// Test case 1: Hybrid mode enabled
	dev.Mode = constants.OktetoHybridModeFieldValue
	executor, err := e.provide(dev, podName, namespace)
	assert.NoError(t, err)
	assert.NotNil(t, executor)
	assert.IsType(t, &hybridExecutor{}, executor)

	// Test case 2: Remote mode enabled
	dev.Mode = constants.OktetoSyncModeFieldValue
	dev.RemotePort = 22000
	executor, err = e.provide(dev, podName, namespace)
	assert.NoError(t, err)
	assert.NotNil(t, executor)
	assert.IsType(t, &sshExecutor{}, executor)

	// Test case 3: Neither hybrid nor remote mode enabled
	dev.Mode = constants.OktetoSyncModeFieldValue
	dev.RemotePort = 0
	t.Setenv("OKTETO_EXECUTE_SSH", "false")
	executor, err = e.provide(dev, podName, namespace)
	assert.NoError(t, err)
	assert.NotNil(t, executor)
	assert.IsType(t, &k8sExecutor{}, executor)
}
