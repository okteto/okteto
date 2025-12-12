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
	"context"
	"testing"

	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/require"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// mockBuildkitClient is a mock implementation of types.BuildkitInterface
type mockBuildkitClient struct {
	responses []*types.BuildKitPodResponse
	errs      []error
	callIndex int
}

func (m *mockBuildkitClient) GetLeastLoadedBuildKitPod(_ context.Context, _ string) (*types.BuildKitPodResponse, error) {
	idx := m.callIndex
	m.callIndex++
	if idx < len(m.errs) && m.errs[idx] != nil {
		return nil, m.errs[idx]
	}
	if idx < len(m.responses) {
		return m.responses[idx], nil
	}
	return &types.BuildKitPodResponse{}, nil
}

// mockOktetoClient is a mock implementation of types.OktetoInterface
type mockOktetoClient struct {
	buildkit types.BuildkitInterface
}

func (m *mockOktetoClient) User() types.UserInterface            { return nil }
func (m *mockOktetoClient) Namespaces() types.NamespaceInterface { return nil }
func (m *mockOktetoClient) Previews() types.PreviewInterface     { return nil }
func (m *mockOktetoClient) Pipeline() types.PipelineInterface    { return nil }
func (m *mockOktetoClient) Stream() types.StreamInterface        { return nil }
func (m *mockOktetoClient) Kubetoken() types.KubetokenInterface  { return nil }
func (m *mockOktetoClient) Buildkit() types.BuildkitInterface    { return m.buildkit }

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
		setupForward  func() *forwarder
		shouldNotFail bool
	}{
		{
			name: "stop with open channel",
			setupForward: func() *forwarder {
				return &forwarder{
					stopChan:  make(chan struct{}, 1),
					readyChan: make(chan struct{}, 1),
					localPort: 8080,
				}
			},
			shouldNotFail: true,
		},
		{
			name: "stop with nil channel",
			setupForward: func() *forwarder {
				return &forwarder{
					stopChan:  nil,
					readyChan: make(chan struct{}, 1),
					localPort: 8080,
				}
			},
			shouldNotFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pf := &PortForwarder{
				forwarder: tt.setupForward(),
				ioCtrl:    io.NewIOController(),
			}

			// Should not panic
			require.NotPanics(t, func() {
				pf.Stop()
			})

			// Verify channel is closed if it was not nil
			if pf.forwarder.stopChan != nil {
				select {
				case _, ok := <-pf.forwarder.stopChan:
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
		forwarder: &forwarder{
			stopChan:  make(chan struct{}, 1),
			readyChan: make(chan struct{}, 1),
			localPort: 8080,
		},
		ioCtrl: io.NewIOController(),
	}

	// First stop should work
	require.NotPanics(t, func() {
		pf.Stop()
	})

	// Verify channel is closed
	select {
	case _, ok := <-pf.forwarder.stopChan:
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
		okCtx: okCtx,
		forwarder: &forwarder{
			localPort: 8443,
		},
		ioCtrl: io.NewIOController(),
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
		okCtx:    okCtx,
		isActive: true,
		forwarder: &forwarder{
			localPort: 8443,
		},
		ioCtrl: io.NewIOController(),
	}

	waiter := NewBuildkitClientWaiter(pf, io.NewIOController())

	require.NotNil(t, waiter)
	require.NotNil(t, waiter.buildkitClientFactory)
}
