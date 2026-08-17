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
	"context"
	"fmt"
	"testing"

	"github.com/okteto/okteto/internal/test"
	forwardk8s "github.com/okteto/okteto/pkg/k8s/forward"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/model/forward"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/ssh"
	"github.com/okteto/okteto/pkg/syncthing"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type recordingForwarder struct {
	added      []forward.Forward
	reserved   []int
	sshPort    int
	sshPortErr error
	started    bool
	stopped    bool
	events     []string
	startedPod string
	startedNS  string
}

func (f *recordingForwarder) Add(v forward.Forward) error {
	f.added = append(f.added, v)
	return nil
}

func (*recordingForwarder) AddReverse(model.Reverse) error { return nil }

func (f *recordingForwarder) Start(pod, namespace string) error {
	f.started = true
	f.startedPod = pod
	f.startedNS = namespace
	f.events = append(f.events, "start")
	return nil
}

type recordingK8sProvider struct {
	requestedConfig *clientcmdapi.Config
	client          kubernetes.Interface
	restConfig      *rest.Config
}

func (p *recordingK8sProvider) Provide(config *clientcmdapi.Config) (kubernetes.Interface, *rest.Config, error) {
	p.requestedConfig = config
	return p.client, p.restConfig, nil
}

type sshForwardsContextKey struct{}

func (*recordingForwarder) StartGlobalForwarding() error { return nil }
func (f *recordingForwarder) Stop()                      { f.stopped = true }

func (f *recordingForwarder) SSHPort() (int, error) {
	return f.sshPort, f.sshPortErr
}

func (f *recordingForwarder) ReserveLocalPort(port int) error {
	f.reserved = append(f.reserved, port)
	f.events = append(f.events, fmt.Sprintf("reserve:%d", port))
	return nil
}

func (*recordingForwarder) TransformLabelsToServiceName(v forward.Forward) (forward.Forward, error) {
	return v, nil
}

func TestGlobalForwarderStartsWhenRequired(t *testing.T) {
	t.Parallel()
	var tests = []struct {
		name             string
		globalFwdSection []forward.GlobalForward
		expectedAnswer   bool
	}{
		{
			name: "is needed global forwarding",
			globalFwdSection: []forward.GlobalForward{
				{
					Local:  8080,
					Remote: 8080,
				},
			},
			expectedAnswer: true,
		},
		{
			name:             "not needed global forwarding",
			globalFwdSection: []forward.GlobalForward{},
			expectedAnswer:   false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			answer := isNeededGlobalForwarder(tt.globalFwdSection)
			assert.Equal(t, answer, tt.expectedAnswer)
		})
	}
}

func TestGlobalForwarderAddsProperlyPortsToForward(t *testing.T) {
	f := ssh.NewForwardManager(context.Background(), ":8080", "0.0.0.0", "0.0.0.0", nil, "test")

	var tests = []struct {
		upContext   *upContext
		name        string
		expectedErr bool
	}{
		{
			name: "add one global forwarder",
			upContext: &upContext{
				Manifest: &model.Manifest{
					GlobalForward: []forward.GlobalForward{
						{
							Local:  8080,
							Remote: 8080,
						},
					},
				},
				Forwarder: f,
			},
			expectedErr: false,
		},
		{
			name: "add two global forwarder",
			upContext: &upContext{
				Manifest: &model.Manifest{
					GlobalForward: []forward.GlobalForward{
						{
							Local:       8081,
							ServiceName: "api",
							Remote:      8080,
						},
						{
							Local:  8082,
							Remote: 8080,
						},
					},
				},
				Forwarder: f,
			},
			expectedErr: false,
		},
		{
			name: "add none global forwarder",
			upContext: &upContext{
				Manifest: &model.Manifest{
					GlobalForward: []forward.GlobalForward{},
				},
				Forwarder: f,
			},
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := addGlobalForwards(tt.upContext)
			if !tt.expectedErr {
				assert.Nil(t, err)
			} else {
				assert.NotNil(t, err)
			}
		})
	}
}

func TestForwards(t *testing.T) {
	tt := []struct {
		clientProvider         okteto.K8sClientProvider
		expected               error
		name                   string
		OktetoExecuteSSHEnvVar string
	}{
		{
			name:                   "fakeClientProvider error",
			OktetoExecuteSSHEnvVar: "false",
			clientProvider: &test.FakeK8sProvider{
				ErrProvide: assert.AnError,
			},
			expected: assert.AnError,
		},
		{
			name:                   "fakeClientProvider error",
			OktetoExecuteSSHEnvVar: "false",
			clientProvider:         test.NewFakeK8sProvider(),
			expected:               fmt.Errorf("port %d is listed multiple times, please check your configuration", 8080),
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			up := &upContext{
				Dev: &model.Dev{
					Forward: []forward.Forward{
						{
							Local:  8080,
							Remote: 8080,
						},
						{
							Local:  8080,
							Remote: 8080,
						},
					},
				},
				K8sClientProvider: tc.clientProvider,
			}
			t.Setenv(model.OktetoExecuteSSHEnvVar, tc.OktetoExecuteSSHEnvVar)
			err := up.forwards(context.Background())
			assert.Equal(t, tc.expected, err)
		})
	}
}

func TestSSHForwarss(t *testing.T) {
	tt := []struct {
		clientProvider okteto.K8sClientProvider
		expected       error
		name           string
	}{
		{
			name: "fakeClientProvider error",
			clientProvider: &test.FakeK8sProvider{
				ErrProvide: assert.AnError,
			},
			expected: assert.AnError,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			up := &upContext{
				Dev: &model.Dev{
					Forward: []forward.Forward{
						{
							Local:  8080,
							Remote: 8080,
						},
						{
							Local:  8080,
							Remote: 8080,
						},
					},
				},
				K8sClientProvider: tc.clientProvider,
			}
			err := up.sshForwards(context.Background())
			assert.ErrorIs(t, tc.expected, err)
		})
	}
}

func TestSSHForwardsWiresConcreteInternalEndpoint(t *testing.T) {
	tests := []struct {
		iface       string
		wantBind    string
		wantDial    string
		wantAddress string
	}{
		{iface: model.Localhost, wantBind: "127.0.0.1", wantDial: "127.0.0.1", wantAddress: "127.0.0.1:0"},
		{iface: "127.0.0.1", wantBind: "127.0.0.1", wantDial: "127.0.0.1", wantAddress: "127.0.0.1:0"},
		{iface: "::1", wantBind: "::1", wantDial: "::1", wantAddress: "[::1]:0"},
		{iface: "0.0.0.0", wantBind: "0.0.0.0", wantDial: "127.0.0.1", wantAddress: "127.0.0.1:0"},
		{iface: "::", wantBind: "::", wantDial: "::1", wantAddress: "[::1]:0"},
		{iface: "192.0.2.10", wantBind: "192.0.2.10", wantDial: "192.0.2.10", wantAddress: "192.0.2.10:0"},
		{iface: "2001:db8::10", wantBind: "2001:db8::10", wantDial: "2001:db8::10", wantAddress: "[2001:db8::10]:0"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("interface=%q", tt.iface), func(t *testing.T) {
			const (
				boundPort        = 23456
				sshServerPort    = 2222
				syncthingPort    = 22000
				syncthingGUIPort = 22001
			)

			recording := &recordingForwarder{sshPort: boundPort}
			clientConfig := &clientcmdapi.Config{CurrentContext: "sentinel-context"}
			restConfig := &rest.Config{Host: "https://sentinel.invalid"}
			k8sClient := k8sfake.NewSimpleClientset()
			provider := &recordingK8sProvider{client: k8sClient, restConfig: restConfig}
			callCtx := context.WithValue(context.Background(), sshForwardsContextKey{}, "sentinel")
			var gotK8sInterface, gotSSHAddr, gotLocalInterface, gotRemoteInterface string
			var gotK8sNamespace, gotSSHNamespace string
			var gotK8sConfig *rest.Config
			var gotK8sClient kubernetes.Interface
			var gotK8sContext, gotSSHContext context.Context
			var gotK8sManager, gotSSHManager *forwardk8s.PortForwardManager
			var gotSSHPortForward forward.Forward
			var gotEntryName, gotEntryHost string
			var gotEntryPort int

			up := &upContext{
				Namespace: "test",
				Dev: &model.Dev{
					Name:          "dev",
					Interface:     tt.iface,
					SSHServerPort: sshServerPort,
				},
				Sy: &syncthing.Syncthing{RemotePort: syncthingPort, RemoteGUIPort: syncthingGUIPort},
				Manifest: &model.Manifest{GlobalForward: []forward.GlobalForward{
					{Local: 41000, Remote: 8080, IsAdded: true},
				}},
				Pod:               &apiv1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "dev-pod"}},
				K8sClientProvider: provider,
				getK8sConfig:      func() *clientcmdapi.Config { return clientConfig },
				newK8sForwardManager: func(ctx context.Context, k8sInterface string, cfg *rest.Config, client kubernetes.Interface, namespace string) *forwardk8s.PortForwardManager {
					gotK8sContext = ctx
					gotK8sInterface = k8sInterface
					gotK8sConfig = cfg
					gotK8sClient = client
					gotK8sNamespace = namespace
					gotK8sManager = forwardk8s.NewPortForwardManager(ctx, k8sInterface, cfg, client, namespace)
					return gotK8sManager
				},
				newSSHForwardManager: func(ctx context.Context, sshAddr, localInterface, remoteInterface string, pf *forwardk8s.PortForwardManager, namespace string) forwarder {
					gotSSHContext = ctx
					gotSSHAddr = sshAddr
					gotLocalInterface = localInterface
					gotRemoteInterface = remoteInterface
					gotSSHManager = pf
					gotSSHNamespace = namespace
					return recording
				},
				addSSHPortForward: func(pf *forwardk8s.PortForwardManager, f forward.Forward) error {
					gotSSHPortForward = f
					return pf.Add(f)
				},
				addToForwarderFn: func(*upContext) error { return nil },
				sshEntryAdder: func(name, host string, port int) error {
					gotEntryName = name
					gotEntryHost = host
					gotEntryPort = port
					return nil
				},
			}

			err := up.sshForwards(callCtx)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantBind, gotK8sInterface)
			assert.Equal(t, tt.wantAddress, gotSSHAddr)
			assert.Equal(t, tt.iface, gotLocalInterface, "user-facing bind interface changed")
			assert.Equal(t, "0.0.0.0", gotRemoteInterface)
			assert.Equal(t, "test", gotK8sNamespace)
			assert.Equal(t, "test", gotSSHNamespace)
			assert.Same(t, clientConfig, provider.requestedConfig)
			assert.Same(t, restConfig, gotK8sConfig)
			assert.Same(t, k8sClient, gotK8sClient)
			assert.Equal(t, callCtx, gotK8sContext)
			assert.Equal(t, callCtx, gotSSHContext)
			assert.Same(t, gotK8sManager, gotSSHManager)
			assert.Equal(t, forward.Forward{Local: 0, Remote: sshServerPort}, gotSSHPortForward)
			assert.Equal(t, "dev", gotEntryName)
			assert.Equal(t, tt.wantDial, gotEntryHost)
			assert.Equal(t, boundPort, gotEntryPort)
			assert.Equal(t, boundPort, up.Dev.RemotePort)
			assert.True(t, recording.started)
			assert.Equal(t, "dev-pod", recording.startedPod)
			assert.Equal(t, "test", recording.startedNS)
			assert.Equal(t, []int{41000}, recording.reserved)
			assert.Equal(t, []string{"reserve:41000", "start"}, recording.events)
			assert.Equal(t, []forward.Forward{
				{Local: syncthingPort, Remote: syncthing.ClusterPort},
				{Local: syncthingGUIPort, Remote: syncthing.GUIPort},
			}, recording.added)
		})
	}
}

func TestSSHForwardsRejectsInvalidInternalPortBeforeConstruction(t *testing.T) {
	for _, port := range []int{-1, 65536} {
		port := port
		t.Run(fmt.Sprintf("port=%d", port), func(t *testing.T) {
			factoryCalled := false
			up := &upContext{
				Dev:               &model.Dev{Interface: model.Localhost, RemotePort: port},
				K8sClientProvider: test.NewFakeK8sProvider(),
				getK8sConfig:      func() *clientcmdapi.Config { return nil },
				newK8sForwardManager: func(context.Context, string, *rest.Config, kubernetes.Interface, string) *forwardk8s.PortForwardManager {
					factoryCalled = true
					return nil
				},
			}

			err := up.sshForwards(context.Background())
			assert.Error(t, err)
			assert.False(t, factoryCalled)
		})
	}
}

func TestSSHForwardsPreservesPinnedPort(t *testing.T) {
	const pinnedPort = 23456
	recording := &recordingForwarder{sshPort: pinnedPort}
	var gotBind, gotAddress, gotEntryHost string
	var gotMapping forward.Forward
	var gotEntryPort int

	up := &upContext{
		Namespace: "test",
		Dev: &model.Dev{
			Name:          "dev",
			Interface:     "0.0.0.0",
			RemotePort:    pinnedPort,
			SSHServerPort: 2222,
		},
		Sy:                &syncthing.Syncthing{RemotePort: 22000, RemoteGUIPort: 22001},
		Manifest:          &model.Manifest{},
		Pod:               &apiv1.Pod{},
		K8sClientProvider: test.NewFakeK8sProvider(),
		getK8sConfig:      func() *clientcmdapi.Config { return nil },
		newK8sForwardManager: func(ctx context.Context, iface string, cfg *rest.Config, client kubernetes.Interface, namespace string) *forwardk8s.PortForwardManager {
			gotBind = iface
			return forwardk8s.NewPortForwardManager(ctx, iface, cfg, client, namespace)
		},
		newSSHForwardManager: func(_ context.Context, address, _, _ string, _ *forwardk8s.PortForwardManager, _ string) forwarder {
			gotAddress = address
			return recording
		},
		addSSHPortForward: func(_ *forwardk8s.PortForwardManager, f forward.Forward) error {
			gotMapping = f
			return nil
		},
		addToForwarderFn: func(*upContext) error { return nil },
		sshEntryAdder: func(_ string, host string, port int) error {
			gotEntryHost = host
			gotEntryPort = port
			return nil
		},
	}

	if err := up.sshForwards(context.Background()); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "0.0.0.0", gotBind)
	assert.Equal(t, "127.0.0.1:23456", gotAddress)
	assert.Equal(t, forward.Forward{Local: pinnedPort, Remote: 2222}, gotMapping)
	assert.Equal(t, "127.0.0.1", gotEntryHost)
	assert.Equal(t, pinnedPort, gotEntryPort)
	assert.Equal(t, pinnedPort, up.Dev.RemotePort)
}

func TestSSHForwardsRejectsInvalidInterfaceBeforeConstruction(t *testing.T) {
	for _, iface := range []string{"", "ssh.example.com", "fe80::1%lo0"} {
		t.Run(fmt.Sprintf("interface=%q", iface), func(t *testing.T) {
			factoryCalled := false
			up := &upContext{
				Dev:               &model.Dev{Interface: iface, RemotePort: 23456},
				K8sClientProvider: test.NewFakeK8sProvider(),
				getK8sConfig:      func() *clientcmdapi.Config { return nil },
				newK8sForwardManager: func(context.Context, string, *rest.Config, kubernetes.Interface, string) *forwardk8s.PortForwardManager {
					factoryCalled = true
					return nil
				},
			}

			if err := up.sshForwards(context.Background()); err == nil {
				t.Fatalf("sshForwards accepted interface %q", iface)
			}
			assert.False(t, factoryCalled)
		})
	}
}

func TestNewSyncExecutorUsesConcreteInternalEndpoint(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		iface    string
		wantHost string
	}{
		{iface: model.Localhost, wantHost: "127.0.0.1"},
		{iface: "0.0.0.0", wantHost: "127.0.0.1"},
		{iface: "::", wantHost: "::1"},
		{iface: "192.0.2.10", wantHost: "192.0.2.10"},
		{iface: "2001:db8::10", wantHost: "2001:db8::10"},
	} {
		t.Run(fmt.Sprintf("interface=%q", tt.iface), func(t *testing.T) {
			t.Parallel()

			executor, err := newSyncExecutor(&upContext{Dev: &model.Dev{Interface: tt.iface, RemotePort: 23456}})
			assert.NoError(t, err)
			assert.Equal(t, tt.wantHost, executor.iface)
			assert.Equal(t, 23456, executor.remotePort)
		})
	}

	for _, iface := range []string{"", "ssh.example.com"} {
		if _, err := newSyncExecutor(&upContext{Dev: &model.Dev{Interface: iface, RemotePort: 23456}}); err == nil {
			t.Fatalf("newSyncExecutor accepted interface %q", iface)
		}
	}
}
