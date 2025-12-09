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
	"errors"
	"net/url"
	"testing"

	"github.com/okteto/okteto/pkg/config"
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

func TestPortForwarder_GetClientFactory(t *testing.T) {
	tests := []struct {
		name             string
		builder          string
		certStr          string
		token            string
		expectedHostname string
		expectNil        bool
	}{
		{
			name:             "valid builder URL",
			builder:          "https://buildkit.example.com:443",
			certStr:          "cert-data",
			token:            "token-data",
			expectedHostname: "buildkit.example.com",
			expectNil:        false,
		},
		{
			name:             "builder URL without port",
			builder:          "https://buildkit.example.com",
			certStr:          "cert-data",
			token:            "token-data",
			expectedHostname: "buildkit.example.com",
			expectNil:        false,
		},
		{
			name:             "builder URL with path",
			builder:          "https://buildkit.example.com/path",
			certStr:          "cert-data",
			token:            "token-data",
			expectedHostname: "buildkit.example.com",
			expectNil:        false,
		},
		{
			name:      "invalid builder URL",
			builder:   "://invalid-url",
			certStr:   "cert-data",
			token:     "token-data",
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			okCtx := &mockPortForwarderOktetoContext{
				builder: tt.builder,
				certStr: tt.certStr,
				token:   tt.token,
			}

			pf := &PortForwarder{
				okCtx: okCtx,
				forwarder: &forwarder{
					localPort: 8443,
				},
				ioCtrl: io.NewIOController(),
			}

			factory := pf.GetClientFactory()

			if tt.expectNil {
				require.Nil(t, factory)
			} else {
				require.NotNil(t, factory)
				require.Equal(t, tt.certStr, factory.cert)
				require.Equal(t, tt.token, factory.token)
				require.Contains(t, factory.builder, "tcp://127.0.0.1")
				require.Equal(t, config.GetCertificatePath(), factory.certificatePath)
				require.Equal(t, tt.expectedHostname, factory.tlsServerName)
			}
		})
	}
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

	waiter := pf.GetWaiter()

	require.NotNil(t, waiter)
	require.NotNil(t, waiter.buildkitClientFactory)
	require.NotNil(t, waiter.logger)
}

func TestPortForwarder_GetClientFactory_ParseURL(t *testing.T) {
	tests := []struct {
		name                string
		builder             string
		expectedTLSHostname string
		expectNil           bool
	}{
		{
			name:                "URL with subdomain",
			builder:             "https://build.okteto.example.com:443",
			expectedTLSHostname: "build.okteto.example.com",
			expectNil:           false,
		},
		{
			name:                "URL with IP address",
			builder:             "https://192.168.1.1:443",
			expectedTLSHostname: "192.168.1.1",
			expectNil:           false,
		},
		{
			name:                "URL with localhost",
			builder:             "https://localhost:443",
			expectedTLSHostname: "localhost",
			expectNil:           false,
		},
		{
			name:                "malformed URL - url.Parse handles gracefully",
			builder:             "https//buildkit.example.com",
			expectedTLSHostname: "",
			expectNil:           false,
		},
		{
			name:                "empty builder URL",
			builder:             "",
			expectedTLSHostname: "",
			expectNil:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			okCtx := &mockPortForwarderOktetoContext{
				builder: tt.builder,
				certStr: "cert",
				token:   "token",
			}

			pf := &PortForwarder{
				okCtx: okCtx,
				forwarder: &forwarder{
					localPort: 9090,
				},
				ioCtrl: io.NewIOController(),
			}

			factory := pf.GetClientFactory()

			if tt.expectNil {
				require.Nil(t, factory)
			} else {
				require.NotNil(t, factory)

				// Verify that the builder was converted to local address
				require.Contains(t, factory.builder, "tcp://127.0.0.1:9090")

				// Verify TLS server name is set correctly (might be empty for malformed URLs)
				require.Equal(t, tt.expectedTLSHostname, factory.tlsServerName)
			}
		})
	}
}

func TestPortForwarder_GetClientFactory_URLHostnameExtraction(t *testing.T) {
	tests := []struct {
		name            string
		builderURL      string
		expectedBuilder string
	}{
		{
			name:            "standard HTTPS URL",
			builderURL:      "https://buildkit.okteto.dev:443",
			expectedBuilder: "tcp://127.0.0.1",
		},
		{
			name:            "HTTP URL",
			builderURL:      "http://buildkit.example.com:80",
			expectedBuilder: "tcp://127.0.0.1",
		},
		{
			name:            "URL without explicit port",
			builderURL:      "https://buildkit.example.com",
			expectedBuilder: "tcp://127.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			okCtx := &mockPortForwarderOktetoContext{
				builder: tt.builderURL,
				certStr: "cert",
				token:   "token",
			}

			pf := &PortForwarder{
				okCtx: okCtx,
				forwarder: &forwarder{
					localPort: 12345,
				},
				ioCtrl: io.NewIOController(),
			}

			factory := pf.GetClientFactory()
			require.NotNil(t, factory)

			// Verify the builder starts with tcp://127.0.0.1
			require.Contains(t, factory.builder, tt.expectedBuilder)

			// Verify it includes the local port
			require.Contains(t, factory.builder, ":12345")
		})
	}
}

func TestPortForwarder_ForwarderStructure(t *testing.T) {
	tests := []struct {
		name      string
		localPort int
	}{
		{
			name:      "standard port",
			localPort: 8080,
		},
		{
			name:      "high port",
			localPort: 65000,
		},
		{
			name:      "low port",
			localPort: 1234,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &forwarder{
				stopChan:  make(chan struct{}, 1),
				readyChan: make(chan struct{}, 1),
				localPort: tt.localPort,
			}

			require.NotNil(t, f.stopChan)
			require.NotNil(t, f.readyChan)
			require.Equal(t, tt.localPort, f.localPort)

			// Verify channels have correct buffer size
			require.Equal(t, 1, cap(f.stopChan))
			require.Equal(t, 1, cap(f.readyChan))
		})
	}
}

func TestGetClientFactory_TLSServerNameConfiguration(t *testing.T) {
	tests := []struct {
		name               string
		builderURL         string
		expectedTLSName    string
		shouldContainLocal bool
	}{
		{
			name:               "production URL",
			builderURL:         "https://buildkit.okteto.com:443",
			expectedTLSName:    "buildkit.okteto.com",
			shouldContainLocal: true,
		},
		{
			name:               "staging URL",
			builderURL:         "https://buildkit.staging.okteto.dev:443",
			expectedTLSName:    "buildkit.staging.okteto.dev",
			shouldContainLocal: true,
		},
		{
			name:               "custom domain",
			builderURL:         "https://buildkit.mycompany.com:8443",
			expectedTLSName:    "buildkit.mycompany.com",
			shouldContainLocal: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			okCtx := &mockPortForwarderOktetoContext{
				builder: tt.builderURL,
				certStr: "certificate-data",
				token:   "access-token",
			}

			pf := &PortForwarder{
				okCtx: okCtx,
				forwarder: &forwarder{
					localPort: 8443,
				},
				ioCtrl: io.NewIOController(),
			}

			factory := pf.GetClientFactory()
			require.NotNil(t, factory)

			// Parse original URL to verify hostname extraction
			originalURL, err := url.Parse(tt.builderURL)
			require.NoError(t, err)

			require.Equal(t, originalURL.Hostname(), tt.expectedTLSName)
			require.Equal(t, tt.expectedTLSName, factory.tlsServerName)

			if tt.shouldContainLocal {
				require.Contains(t, factory.builder, "127.0.0.1")
			}
		})
	}
}

func TestPortForwarder_ChannelManagement(t *testing.T) {
	t.Run("stop closes channel only once", func(t *testing.T) {
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
	})

	t.Run("ready channel remains open after stop", func(t *testing.T) {
		pf := &PortForwarder{
			forwarder: &forwarder{
				stopChan:  make(chan struct{}, 1),
				readyChan: make(chan struct{}, 1),
				localPort: 8080,
			},
			ioCtrl: io.NewIOController(),
		}

		pf.Stop()

		// readyChan should still be open (only stopChan is closed)
		select {
		case <-pf.forwarder.readyChan:
			t.Error("readyChan should not have a value")
		default:
			// This is expected - channel is open but empty
		}
	})
}

func TestPortForwarder_ConstructedAddressFormat(t *testing.T) {
	tests := []struct {
		name              string
		localPort         int
		expectedSubstring string
	}{
		{
			name:              "port 8080",
			localPort:         8080,
			expectedSubstring: "tcp://127.0.0.1:8080",
		},
		{
			name:              "port 1234",
			localPort:         1234,
			expectedSubstring: "tcp://127.0.0.1:1234",
		},
		{
			name:              "port 50000",
			localPort:         50000,
			expectedSubstring: "tcp://127.0.0.1:50000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			okCtx := &mockPortForwarderOktetoContext{
				builder: "https://buildkit.example.com",
				certStr: "cert",
				token:   "token",
			}

			pf := &PortForwarder{
				okCtx: okCtx,
				forwarder: &forwarder{
					localPort: tt.localPort,
				},
				ioCtrl: io.NewIOController(),
			}

			factory := pf.GetClientFactory()
			require.NotNil(t, factory)
			require.Equal(t, tt.expectedSubstring, factory.builder)
		})
	}
}

func TestPortForwarder_CertificatePathConfiguration(t *testing.T) {
	okCtx := &mockPortForwarderOktetoContext{
		builder: "https://buildkit.example.com",
		certStr: "base64-encoded-cert",
		token:   "auth-token",
	}

	pf := &PortForwarder{
		okCtx: okCtx,
		forwarder: &forwarder{
			localPort: 8443,
		},
		ioCtrl: io.NewIOController(),
	}

	factory := pf.GetClientFactory()
	require.NotNil(t, factory)

	expectedCertPath := config.GetCertificatePath()
	require.Equal(t, expectedCertPath, factory.certificatePath)
	require.NotEmpty(t, factory.certificatePath)
}

func TestPortForwarder_GetWaiter_Configuration(t *testing.T) {
	okCtx := &mockPortForwarderOktetoContext{
		builder: "https://buildkit.example.com:443",
		certStr: "certificate",
		token:   "token",
	}

	pf := &PortForwarder{
		okCtx: okCtx,
		forwarder: &forwarder{
			localPort: 8443,
		},
		ioCtrl: io.NewIOController(),
	}

	waiter := pf.GetWaiter()

	require.NotNil(t, waiter)
	require.NotNil(t, waiter.buildkitClientFactory)

	// Verify the waiter's factory is properly configured
	factory, ok := waiter.buildkitClientFactory.(*buildkitClientFactoryToWait)
	require.True(t, ok, "buildkitClientFactory should be of type *buildkitClientFactoryToWait")
	require.NotNil(t, factory.factory)
}

func TestPortForwarder_WaitUntilIsReady_ReusesExistingConnection(t *testing.T) {
	tests := []struct {
		name      string
		isActive  bool
		localPort int
	}{
		{
			name:      "reuses connection on port 8080",
			isActive:  true,
			localPort: 8080,
		},
		{
			name:      "reuses connection on port 9999",
			localPort: 9999,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pf := &PortForwarder{
				isActive:  true,
				sessionID: "test-session",
				forwarder: &forwarder{
					localPort: tt.localPort,
					stopChan:  make(chan struct{}, 1),
					readyChan: make(chan struct{}, 1),
				},
				ioCtrl: io.NewIOController(),
			}

			err := pf.WaitUntilIsReady(context.Background())

			require.NoError(t, err)
			require.True(t, pf.isActive)
		})
	}
}

func TestPortForwarder_WaitUntilIsReady_ErrorFromBuildkitClient(t *testing.T) {
	tests := []struct {
		name          string
		buildkitErr   error
		expectedError string
	}{
		{
			name:          "connection refused error",
			buildkitErr:   errors.New("connection refused"),
			expectedError: "could not get least loaded buildkit pod: connection refused",
		},
		{
			name:          "timeout error",
			buildkitErr:   errors.New("context deadline exceeded"),
			expectedError: "could not get least loaded buildkit pod: context deadline exceeded",
		},
		{
			name:          "unauthorized error",
			buildkitErr:   errors.New("unauthorized"),
			expectedError: "could not get least loaded buildkit pod: unauthorized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockBuildkit := &mockBuildkitClient{
				errs: []error{tt.buildkitErr},
			}
			mockClient := &mockOktetoClient{buildkit: mockBuildkit}

			pf := &PortForwarder{
				isActive:     false,
				sessionID:    "test-session",
				oktetoClient: mockClient,
				forwarder: &forwarder{
					localPort: 8080,
					stopChan:  make(chan struct{}, 1),
					readyChan: make(chan struct{}, 1),
				},
				ioCtrl: io.NewIOController(),
			}

			err := pf.WaitUntilIsReady(context.Background())

			require.Error(t, err)
			require.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestPortForwarder_WaitUntilIsReady_ContextCancelledWhilePolling(t *testing.T) {
	tests := []struct {
		name           string
		queuePosition  int
		totalInQueue   int
		expectedReason string
	}{
		{
			name:           "cancelled while first in queue",
			queuePosition:  1,
			totalInQueue:   5,
			expectedReason: "context cancelled while waiting for buildkit pod",
		},
		{
			name:           "cancelled while third in queue",
			queuePosition:  3,
			totalInQueue:   10,
			expectedReason: "context cancelled while waiting for buildkit pod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockBuildkit := &mockBuildkitClient{
				responses: []*types.BuildKitPodResponse{
					{
						PodName:       "",
						QueuePosition: tt.queuePosition,
						TotalInQueue:  tt.totalInQueue,
						Reason:        "waiting for resources",
					},
				},
			}
			mockClient := &mockOktetoClient{buildkit: mockBuildkit}

			ctx, cancel := context.WithCancel(context.Background())
			cancel() // Cancel immediately

			pf := &PortForwarder{
				isActive:     false,
				sessionID:    "test-session",
				oktetoClient: mockClient,
				forwarder: &forwarder{
					localPort: 8080,
					stopChan:  make(chan struct{}, 1),
					readyChan: make(chan struct{}, 1),
				},
				ioCtrl: io.NewIOController(),
			}

			err := pf.WaitUntilIsReady(ctx)

			require.Error(t, err)
			require.Contains(t, err.Error(), tt.expectedReason)
		})
	}
}

func TestPortForwarder_WaitUntilIsReady_QueuePolling_ContextCancelled(t *testing.T) {
	tests := []struct {
		name          string
		queuePosition int
		totalInQueue  int
		reason        string
	}{
		{
			name:          "first in queue then cancelled",
			queuePosition: 1,
			totalInQueue:  3,
			reason:        "waiting for resources",
		},
		{
			name:          "fifth in queue then cancelled",
			queuePosition: 5,
			totalInQueue:  10,
			reason:        "scaling up",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			// Cancel immediately - the poll loop should detect this
			cancel()

			mockBuildkit := &mockBuildkitClient{
				responses: []*types.BuildKitPodResponse{
					// Always return queue status - never a ready pod
					{
						PodName:       "",
						QueuePosition: tt.queuePosition,
						TotalInQueue:  tt.totalInQueue,
						Reason:        tt.reason,
					},
				},
			}
			mockClient := &mockOktetoClient{buildkit: mockBuildkit}

			pf := &PortForwarder{
				isActive:     false,
				sessionID:    "test-session",
				oktetoClient: mockClient,
				okCtx: &mockPortForwarderOktetoContext{
					globalNamespace: "okteto",
				},
				forwarder: &forwarder{
					localPort: 8080,
					stopChan:  make(chan struct{}, 1),
					readyChan: make(chan struct{}, 1),
				},
				ioCtrl: io.NewIOController(),
			}

			err := pf.WaitUntilIsReady(ctx)

			require.Error(t, err)
			require.Contains(t, err.Error(), "context cancelled")
		})
	}
}

func TestPortForwarder_WaitUntilIsReady_MultiplePolls_BeforeError(t *testing.T) {
	tests := []struct {
		name           string
		pollsBeforeErr int
		buildkitErr    error
	}{
		{
			name:           "error after 1 poll",
			pollsBeforeErr: 1,
			buildkitErr:    errors.New("service unavailable"),
		},
		{
			name:           "error after 2 polls",
			pollsBeforeErr: 2,
			buildkitErr:    errors.New("internal error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			responses := make([]*types.BuildKitPodResponse, tt.pollsBeforeErr)
			errs := make([]error, tt.pollsBeforeErr+1)

			for i := range tt.pollsBeforeErr {
				responses[i] = &types.BuildKitPodResponse{
					PodName:       "",
					QueuePosition: i + 1,
					TotalInQueue:  5,
					Reason:        "waiting",
				}
				errs[i] = nil
			}
			errs[tt.pollsBeforeErr] = tt.buildkitErr

			mockBuildkit := &mockBuildkitClient{
				responses: responses,
				errs:      errs,
			}
			mockClient := &mockOktetoClient{buildkit: mockBuildkit}

			pf := &PortForwarder{
				isActive:     false,
				sessionID:    "test-session",
				oktetoClient: mockClient,
				okCtx: &mockPortForwarderOktetoContext{
					globalNamespace: "okteto",
				},
				forwarder: &forwarder{
					localPort: 8080,
					stopChan:  make(chan struct{}, 1),
					readyChan: make(chan struct{}, 1),
				},
				ioCtrl: io.NewIOController(),
			}

			err := pf.WaitUntilIsReady(context.Background())

			require.Error(t, err)
			require.Contains(t, err.Error(), "could not get least loaded buildkit pod")
			require.Equal(t, tt.pollsBeforeErr+1, mockBuildkit.callIndex)
		})
	}
}
