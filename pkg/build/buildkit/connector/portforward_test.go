// Copyright 2025 The Okteto Authors
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

package connector

import (
	"testing"

	"github.com/okteto/okteto/pkg/log/io"
	"github.com/stretchr/testify/require"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// mockPortForwarderOktetoContext is a mock implementation of PortForwarderOktetoContextInterface
type mockPortForwarderOktetoContext struct {
	certStr         string
	builder         string
	token           string
	namespace       string
	globalNamespace string
	currentUser     string
	currentCfg      *clientcmdapi.Config
}

func (m *mockPortForwarderOktetoContext) GetCurrentCertStr() string {
	return m.certStr
}

func (m *mockPortForwarderOktetoContext) GetCurrentBuilder() string {
	return m.builder
}

func (m *mockPortForwarderOktetoContext) GetCurrentToken() string {
	return m.token
}

func (m *mockPortForwarderOktetoContext) GetNamespace() string {
	return m.namespace
}

func (m *mockPortForwarderOktetoContext) GetGlobalNamespace() string {
	return m.globalNamespace
}

func (m *mockPortForwarderOktetoContext) GetCurrentUser() string {
	return m.currentUser
}

func (m *mockPortForwarderOktetoContext) GetCurrentCfg() *clientcmdapi.Config {
	return m.currentCfg
}

func TestPortForwarder_Stop(t *testing.T) {
	tests := []struct {
		name          string
		stopChan      chan struct{}
		shouldNotFail bool
	}{
		{
			name:          "stop with open channel",
			stopChan:      make(chan struct{}, 1),
			shouldNotFail: true,
		},
		{
			name:          "stop with nil channel",
			stopChan:      nil,
			shouldNotFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pf := &PortForwarder{
				stopChan:  tt.stopChan,
				readyChan: make(chan struct{}, 1),
				localPort: 8080,
				ioCtrl:    io.NewIOController(),
			}

			// Should not panic
			require.NotPanics(t, func() {
				pf.Stop()
			})

			// Verify channel is closed if it was not nil
			if pf.stopChan != nil {
				select {
				case _, ok := <-pf.stopChan:
					require.False(t, ok, "channel should be closed")
				default:
					t.Error("channel should be closed but is still open")
				}
			}
		})
	}
}

func TestPortForwarder_Stop_MultipleCallsSafe(t *testing.T) {
	pf := &PortForwarder{
		stopChan:  make(chan struct{}, 1),
		readyChan: make(chan struct{}, 1),
		localPort: 8080,
		ioCtrl:    io.NewIOController(),
	}

	// First stop should work
	require.NotPanics(t, func() {
		pf.Stop()
	})

	// Verify channel is closed
	select {
	case _, ok := <-pf.stopChan:
		require.False(t, ok, "channel should be closed")
	default:
		t.Error("channel should be closed")
	}

	// Second stop should also not panic (idempotent)
	require.NotPanics(t, func() {
		pf.Stop()
	})

	// Third stop should still not panic
	require.NotPanics(t, func() {
		pf.Stop()
	})
}
func TestPortForwarder_GetWaiter(t *testing.T) {
	okCtx := &mockPortForwarderOktetoContext{
		builder: "https://buildkit.example.com",
		certStr: "cert-data",
		token:   "token-data",
	}

	pf := &PortForwarder{
		okCtx:     okCtx,
		localPort: 8443,
		ioCtrl:    io.NewIOController(),
	}

	waiter := NewBuildkitClientWaiter(pf, io.NewIOController())

	require.NotNil(t, waiter)
	require.NotNil(t, waiter.buildkitClientFactory)
	require.NotNil(t, waiter.logger)
}

func TestPortForwarder_GetWaiter_Configuration(t *testing.T) {
	okCtx := &mockPortForwarderOktetoContext{
		builder: "https://buildkit.example.com:443",
		certStr: "certificate",
		token:   "token",
	}

	pf := &PortForwarder{
		okCtx:     okCtx,
		isActive:  true,
		localPort: 8443,
		ioCtrl:    io.NewIOController(),
	}

	waiter := NewBuildkitClientWaiter(pf, io.NewIOController())

	require.NotNil(t, waiter)
	require.NotNil(t, waiter.buildkitClientFactory)
}
