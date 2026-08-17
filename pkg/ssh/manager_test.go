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

package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/okteto/okteto/internal/sshtransport"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	forwardModel "github.com/okteto/okteto/pkg/model/forward"
	cryptossh "golang.org/x/crypto/ssh"
	"k8s.io/client-go/tools/portforward"
)

type testHTTPHandler struct {
	message string
}
type testSSHHandler struct{}

type fakeKubePortForwarder struct {
	mu             sync.Mutex
	startErr       error
	portErrs       []error
	ports          [][]portforward.ForwardedPort
	order          *[]string
	onStart        func(int)
	start          func(context.Context) error
	onForwarded    func(int) ([]portforward.ForwardedPort, error)
	starts         int
	stops          int
	forwardedCalls int
}

func (f *fakeKubePortForwarder) StartContext(ctx context.Context, _ string, _ string) error {
	f.mu.Lock()
	f.starts++
	attempt := f.starts
	f.mu.Unlock()
	if f.onStart != nil {
		f.onStart(attempt)
	}
	if f.order != nil {
		*f.order = append(*f.order, "start")
	}
	if f.start != nil {
		return f.start(ctx)
	}
	return f.startErr
}

func (f *fakeKubePortForwarder) ForwardedPorts() ([]portforward.ForwardedPort, error) {
	f.mu.Lock()
	f.forwardedCalls++
	call := f.forwardedCalls
	starts := f.starts
	f.mu.Unlock()
	if f.order != nil {
		*f.order = append(*f.order, "ports")
	}
	if f.onForwarded != nil {
		return f.onForwarded(call)
	}
	index := starts - 1
	var err error
	if index >= 0 && index < len(f.portErrs) {
		err = f.portErrs[index]
	}
	if index < 0 || index >= len(f.ports) {
		return nil, err
	}
	return f.ports[index], err
}

func (f *fakeKubePortForwarder) Stop() {
	f.mu.Lock()
	f.stops++
	f.mu.Unlock()
}

func (*fakeKubePortForwarder) GetServiceNameByLabel(string, map[string]string) (string, error) {
	return "", nil
}

func TestForwardManagerRejectsListenerLossDuringPoolSetup(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("listener stopped")
	ctx, cancel := context.WithCancel(context.Background())
	pf := &fakeKubePortForwarder{}
	pf.onForwarded = func(call int) ([]portforward.ForwardedPort, error) {
		if call == 1 {
			return []portforward.ForwardedPort{{Local: 34567, Remote: 2222}}, nil
		}
		return nil, sentinel
	}
	fm := newForwardManager(ctx, "127.0.0.1:0", model.Localhost, "0.0.0.0", pf, "test")
	fm.getClientConfig = func() (*cryptossh.ClientConfig, error) { return &cryptossh.ClientConfig{}, nil }
	fm.startPool = func(context.Context, context.Context, string, *cryptossh.ClientConfig) (*pool, error) {
		cancel()
		return &pool{}, nil
	}

	if err := fm.Start("pod", "namespace"); err == nil {
		t.Fatal("Start accepted a port-forward that died during pool setup")
	}
	if fm.pool != nil {
		t.Fatal("dead port-forward published a connection pool")
	}
	if pf.stops != 1 {
		t.Fatalf("dead port-forward stopped %d times, want 1", pf.stops)
	}
}

func TestForwardManagerStopPreventsPoolPublication(t *testing.T) {
	t.Parallel()

	secondLivenessCheck := make(chan struct{})
	releaseLivenessCheck := make(chan struct{})
	pf := &fakeKubePortForwarder{}
	pf.onForwarded = func(call int) ([]portforward.ForwardedPort, error) {
		if call == 2 {
			close(secondLivenessCheck)
			<-releaseLivenessCheck
		}
		return []portforward.ForwardedPort{{Local: 34567, Remote: 2222}}, nil
	}
	fm := newForwardManager(context.Background(), "127.0.0.1:0", model.Localhost, "0.0.0.0", pf, "test")
	fm.getClientConfig = func() (*cryptossh.ClientConfig, error) { return &cryptossh.ClientConfig{}, nil }
	candidate := &pool{}
	fm.startPool = func(context.Context, context.Context, string, *cryptossh.ClientConfig) (*pool, error) {
		return candidate, nil
	}

	startResult := make(chan error, 1)
	go func() {
		startResult <- fm.Start("pod", "namespace")
	}()
	<-secondLivenessCheck
	fm.Stop()
	close(releaseLivenessCheck)

	if err := <-startResult; err == nil {
		t.Fatal("Start published a connection pool after Stop")
	}
	if fm.pool != nil {
		t.Fatal("stopped manager retained a published connection pool")
	}
	if !candidate.stopped.Load() {
		t.Fatal("unpublished connection pool was not closed")
	}
	if pf.stops != 2 {
		t.Fatalf("port-forward stopped %d times, want once by Stop and once by failed Start", pf.stops)
	}
}

func TestForwardManagerStopCancelsStartupWithoutRetry(t *testing.T) {
	t.Parallel()

	poolStarted := make(chan struct{})
	pf := &fakeKubePortForwarder{ports: [][]portforward.ForwardedPort{{{Local: 34567, Remote: 2222}}}}
	fm := newForwardManager(context.Background(), "127.0.0.1:0", model.Localhost, "0.0.0.0", pf, "test")
	fm.getClientConfig = func() (*cryptossh.ClientConfig, error) {
		return &cryptossh.ClientConfig{Timeout: time.Minute}, nil
	}
	fm.startPool = func(_ context.Context, connectionCtx context.Context, _ string, _ *cryptossh.ClientConfig) (*pool, error) {
		close(poolStarted)
		<-connectionCtx.Done()
		return nil, connectionCtx.Err()
	}

	result := make(chan error, 1)
	go func() { result <- fm.Start("pod", "namespace") }()
	<-poolStarted
	fm.Stop()

	select {
	case err := <-result:
		if err == nil {
			t.Fatal("Start succeeded after Stop cancelled startup")
		}
	case <-time.After(time.Second):
		t.Fatal("Stop did not cancel in-flight startup")
	}
	if pf.starts != 1 {
		t.Fatalf("port-forward started %d times after Stop, want 1", pf.starts)
	}
}

func TestForwardManagerBoundsKubePortForwardStartup(t *testing.T) {
	t.Parallel()

	lifetimeCtx, cancelLifetime := context.WithCancel(context.Background())
	defer cancelLifetime()
	pf := &fakeKubePortForwarder{
		start: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}
	fm := newForwardManager(lifetimeCtx, "127.0.0.1:0", model.Localhost, "0.0.0.0", pf, "test")
	fm.getClientConfig = func() (*cryptossh.ClientConfig, error) {
		return &cryptossh.ClientConfig{Timeout: 20 * time.Millisecond}, nil
	}

	started := time.Now()
	result := make(chan error, 1)
	go func() { result <- fm.Start("pod", "namespace") }()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("Start accepted a Kubernetes port-forward that exceeded the startup timeout")
		}
	case <-time.After(time.Second):
		cancelLifetime()
		fm.Stop()
		t.Fatal("Kubernetes port-forward startup ignored its bounded context")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("Kubernetes port-forward startup took %s after a 20ms timeout", elapsed)
	}
	if pf.stops != 1 {
		t.Fatalf("timed-out Kubernetes port-forward stopped %d times, want 1", pf.stops)
	}
}

func TestForwardManagerStartUsesResolvedPortBeforePool(t *testing.T) {
	t.Parallel()

	order := []string{}
	pf := &fakeKubePortForwarder{
		order: &order,
		ports: [][]portforward.ForwardedPort{{{Local: 34567, Remote: 2222}}},
	}
	fm := newForwardManager(context.Background(), "127.0.0.1:0", model.Localhost, "0.0.0.0", pf, "test")
	fm.getClientConfig = func() (*cryptossh.ClientConfig, error) {
		order = append(order, "config")
		return &cryptossh.ClientConfig{}, nil
	}
	var poolAddress string
	var lifetimeCtx, connectionCtx context.Context
	fm.startPool = func(lifetime, connection context.Context, address string, _ *cryptossh.ClientConfig) (*pool, error) {
		order = append(order, "pool")
		poolAddress = address
		lifetimeCtx = lifetime
		connectionCtx = connection
		return &pool{}, nil
	}

	if err := fm.Start("pod", "namespace"); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(order, ","); got != "config,start,ports,pool,ports" {
		t.Fatalf("startup order = %q", got)
	}
	if poolAddress != "127.0.0.1:34567" {
		t.Fatalf("pool address = %q, want 127.0.0.1:34567", poolAddress)
	}
	if port, err := fm.SSHPort(); err != nil || port != 34567 {
		t.Fatalf("SSHPort() = %d, %v; want 34567, nil", port, err)
	}
	if pf.stops != 0 {
		t.Fatalf("successful port-forward was stopped %d times", pf.stops)
	}
	select {
	case <-lifetimeCtx.Done():
		t.Fatal("successful startup cancelled the pool lifetime context")
	default:
	}
	select {
	case <-connectionCtx.Done():
	default:
		t.Fatal("startup connection context was not released")
	}
}

func TestForwardManagerHonorsConfiguredStartupTimeout(t *testing.T) {
	t.Parallel()

	pf := &fakeKubePortForwarder{ports: [][]portforward.ForwardedPort{{{Local: 34567, Remote: 2222}}}}
	fm := newForwardManager(context.Background(), "127.0.0.1:0", model.Localhost, "0.0.0.0", pf, "test")
	fm.getClientConfig = func() (*cryptossh.ClientConfig, error) {
		return &cryptossh.ClientConfig{Timeout: 30 * time.Second}, nil
	}
	var remaining time.Duration
	fm.startPool = func(_ context.Context, connectionCtx context.Context, _ string, _ *cryptossh.ClientConfig) (*pool, error) {
		deadline, ok := connectionCtx.Deadline()
		if !ok {
			t.Fatal("connection context has no deadline")
		}
		remaining = time.Until(deadline)
		return &pool{}, nil
	}

	if err := fm.Start("pod", "namespace"); err != nil {
		t.Fatal(err)
	}
	if remaining < 25*time.Second {
		t.Fatalf("configured 30s timeout was truncated to %s", remaining)
	}
}

func TestForwardManagerStartRejectsInvalidResolvedPortsBeforePool(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("get ports failed")
	tests := []struct {
		name     string
		address  string
		ports    []portforward.ForwardedPort
		portsErr error
	}{
		{name: "get ports error", address: "127.0.0.1:0", portsErr: sentinel},
		{name: "no ports", address: "127.0.0.1:0"},
		{name: "multiple ports", address: "127.0.0.1:0", ports: []portforward.ForwardedPort{{Local: 30001}, {Local: 30002}}},
		{name: "unresolved zero", address: "127.0.0.1:0", ports: []portforward.ForwardedPort{{Local: 0}}},
		{name: "pinned mismatch", address: "127.0.0.1:30001", ports: []portforward.ForwardedPort{{Local: 30002}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pf := &fakeKubePortForwarder{ports: [][]portforward.ForwardedPort{tt.ports}, portErrs: []error{tt.portsErr}}
			fm := newForwardManager(context.Background(), tt.address, model.Localhost, "0.0.0.0", pf, "test")
			fm.getClientConfig = func() (*cryptossh.ClientConfig, error) { return &cryptossh.ClientConfig{}, nil }
			poolCalled := false
			fm.startPool = func(context.Context, context.Context, string, *cryptossh.ClientConfig) (*pool, error) {
				poolCalled = true
				return &pool{}, nil
			}

			if err := fm.Start("pod", "namespace"); err == nil {
				t.Fatal("Start accepted an invalid resolved endpoint")
			}
			if poolCalled {
				t.Fatal("connection pool started with an invalid resolved endpoint")
			}
			if pf.stops != 1 {
				t.Fatalf("invalid port-forward stopped %d times, want 1", pf.stops)
			}
		})
	}
}

func TestForwardManagerRetriesAutomaticPortCollision(t *testing.T) {
	t.Parallel()

	pf := &fakeKubePortForwarder{ports: [][]portforward.ForwardedPort{
		{{Local: 34567, Remote: 2222}},
		{{Local: 34568, Remote: 2222}},
	}}
	fm := newForwardManager(context.Background(), "127.0.0.1:0", model.Localhost, "0.0.0.0", pf, "test")
	if err := fm.ReserveLocalPort(34567); err != nil {
		t.Fatal(err)
	}
	fm.getClientConfig = func() (*cryptossh.ClientConfig, error) { return &cryptossh.ClientConfig{}, nil }
	var poolAddress string
	fm.startPool = func(_ context.Context, _ context.Context, address string, _ *cryptossh.ClientConfig) (*pool, error) {
		poolAddress = address
		return &pool{}, nil
	}

	if err := fm.Start("pod", "namespace"); err != nil {
		t.Fatal(err)
	}
	if pf.starts != 2 || pf.stops != 1 {
		t.Fatalf("retry lifecycle = %d starts, %d stops; want 2, 1", pf.starts, pf.stops)
	}
	if poolAddress != "127.0.0.1:34568" {
		t.Fatalf("pool address = %q, want 127.0.0.1:34568", poolAddress)
	}
	if err := fm.Add(forwardModel.Forward{Local: 34568, Remote: 8080, IsGlobal: true}); err == nil {
		t.Fatal("late global forward reused the resolved SSH port")
	}
}

func TestForwardManagerAutomaticCollisionRetryIsCancellable(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	pf := &fakeKubePortForwarder{ports: [][]portforward.ForwardedPort{
		{{Local: 34567, Remote: 2222}},
		{{Local: 34567, Remote: 2222}},
	}}
	pf.onStart = func(attempt int) {
		if attempt == 2 {
			cancel()
		}
	}
	fm := newForwardManager(ctx, "127.0.0.1:0", model.Localhost, "0.0.0.0", pf, "test")
	if err := fm.ReserveLocalPort(34567); err != nil {
		t.Fatal(err)
	}
	fm.getClientConfig = func() (*cryptossh.ClientConfig, error) { return &cryptossh.ClientConfig{}, nil }
	poolCalled := false
	fm.startPool = func(context.Context, context.Context, string, *cryptossh.ClientConfig) (*pool, error) {
		poolCalled = true
		return &pool{}, nil
	}

	if err := fm.Start("pod", "namespace"); err == nil {
		t.Fatal("Start ignored cancellation during repeated automatic-port collisions")
	}
	if pf.starts != 2 || pf.stops != 2 {
		t.Fatalf("collision cancellation lifecycle = %d starts, %d stops; want 2, 2", pf.starts, pf.stops)
	}
	if poolCalled {
		t.Fatal("pool started during repeated port collisions")
	}
}

func TestForwardManagerPreservesNilKubeForwarder(t *testing.T) {
	t.Parallel()

	fm := NewForwardManager(context.Background(), "127.0.0.1:2222", model.Localhost, "0.0.0.0", nil, "")
	if fm.pf != nil {
		t.Fatal("nil Kubernetes forwarder became a non-nil interface")
	}
}

func TestForwardManagerRejectsPinnedSSHPortAsUserForward(t *testing.T) {
	t.Parallel()

	fm := NewForwardManager(context.Background(), "127.0.0.1:23456", model.Localhost, "0.0.0.0", nil, "")
	if err := fm.Add(forwardModel.Forward{Local: 23456, Remote: 8080}); err == nil {
		t.Fatal("user forward reused pinned SSH port")
	}
	if err := fm.AddReverse(model.Reverse{Local: 23456, Remote: 8080}); err == nil {
		t.Fatal("reverse forward reused pinned SSH port")
	}
}

func TestForwardManagerReservationsRejectConflicts(t *testing.T) {
	t.Parallel()

	fm := NewForwardManager(context.Background(), "127.0.0.1:23456", model.Localhost, "0.0.0.0", nil, "")
	for _, port := range []int{0, 65536, 23456} {
		if err := fm.ReserveLocalPort(port); err == nil {
			t.Fatalf("ReserveLocalPort(%d) accepted an invalid or pinned SSH port", port)
		}
	}
	if err := fm.ReserveLocalPort(34567); err != nil {
		t.Fatal(err)
	}
	if err := fm.ReserveLocalPort(34567); err == nil {
		t.Fatal("duplicate reservation was accepted")
	}
	if err := fm.Add(forwardModel.Forward{Local: 34567, Remote: 8080}); err == nil {
		t.Fatal("normal forward consumed a global-forward reservation")
	}
	if err := fm.AddReverse(model.Reverse{Local: 34567, Remote: 8080}); err == nil {
		t.Fatal("reverse forward consumed a global-forward reservation")
	}
}

func TestForwardManagerRejectsResolvedPortCollisions(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"forward", "global", "reverse"} {
		t.Run(kind, func(t *testing.T) {
			fm := NewForwardManager(context.Background(), "127.0.0.1:0", model.Localhost, "0.0.0.0", nil, "")
			switch kind {
			case "forward":
				fm.forwards[34567] = &forward{}
			case "global":
				fm.globalForwards[34567] = &forward{}
			case "reverse":
				fm.reverses[34567] = &reverse{}
			}
			if err := fm.setSSHAddressFromForwardedPorts([]portforward.ForwardedPort{{Local: 34567, Remote: 2222}}); !errors.Is(err, errAutomaticSSHPortCollision) {
				t.Fatalf("resolved %s collision error = %v, want errAutomaticSSHPortCollision", kind, err)
			}
		})
	}
}

func TestForwardManagerCannotReserveExistingForwardPorts(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"forward", "global", "reverse"} {
		t.Run(kind, func(t *testing.T) {
			fm := NewForwardManager(context.Background(), "127.0.0.1:0", model.Localhost, "0.0.0.0", nil, "")
			switch kind {
			case "forward":
				fm.forwards[34567] = &forward{}
			case "global":
				fm.globalForwards[34567] = &forward{}
			case "reverse":
				fm.reverses[34567] = &reverse{}
			}
			if err := fm.ReserveLocalPort(34567); err == nil {
				t.Fatalf("reservation duplicated an existing %s port", kind)
			}
		})
	}
}

func TestForwardManagerGlobalForwardConsumesItsReservation(t *testing.T) {
	t.Parallel()

	fm := NewForwardManager(context.Background(), "127.0.0.1:0", model.Localhost, "0.0.0.0", nil, "")
	fm.isPortAvailable = func(iface string, port int) bool {
		return iface == model.Localhost && port == 34567
	}
	if err := fm.ReserveLocalPort(34567); err != nil {
		t.Fatal(err)
	}
	if err := fm.Add(forwardModel.Forward{Local: 34567, Remote: 8080, IsGlobal: true}); err != nil {
		t.Fatalf("global forward could not consume its reservation: %v", err)
	}
	if _, reserved := fm.reservedLocalPorts[34567]; reserved {
		t.Fatal("global forward left its reservation behind")
	}
	if _, added := fm.globalForwards[34567]; !added {
		t.Fatal("global forward was not added")
	}
}

func TestForwardManagerResolvesAtomicallyAllocatedSSHPort(t *testing.T) {
	t.Parallel()

	fm := NewForwardManager(context.Background(), "127.0.0.1:0", model.Localhost, "0.0.0.0", nil, "test")
	err := fm.setSSHAddressFromForwardedPorts([]portforward.ForwardedPort{{Local: 34567, Remote: 2222}})
	if err != nil {
		t.Fatal(err)
	}
	if fm.sshAddr != "127.0.0.1:34567" {
		t.Fatalf("SSH address = %q, want 127.0.0.1:34567", fm.sshAddr)
	}
	port, err := fm.SSHPort()
	if err != nil {
		t.Fatal(err)
	}
	if port != 34567 {
		t.Fatalf("SSH port = %d, want 34567", port)
	}
}

func TestForwardManagerRejectsAmbiguousOrUnsafeResolvedSSHPort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		address string
		ports   []portforward.ForwardedPort
	}{
		{name: "no ports", address: "127.0.0.1:0"},
		{name: "multiple ports", address: "127.0.0.1:0", ports: []portforward.ForwardedPort{{Local: 30001}, {Local: 30002}}},
		{name: "zero remains unresolved", address: "127.0.0.1:0", ports: []portforward.ForwardedPort{{Local: 0}}},
		{name: "hostname destination", address: "ssh.example.com:0", ports: []portforward.ForwardedPort{{Local: 30001}}},
		{name: "malformed destination", address: ":", ports: []portforward.ForwardedPort{{Local: 30001}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fm := NewForwardManager(context.Background(), tt.address, model.Localhost, "0.0.0.0", nil, "test")
			if err := fm.setSSHAddressFromForwardedPorts(tt.ports); err == nil {
				t.Fatalf("accepted unsafe or ambiguous endpoint %q with ports %+v", tt.address, tt.ports)
			}
		})
	}
}

func TestForwardManagerSSHPortRejectsNonConcreteEndpoint(t *testing.T) {
	t.Parallel()

	for _, address := range []string{"127.0.0.1:0", "127.0.0.1:65536", "ssh.example.com:2222"} {
		address := address
		t.Run(address, func(t *testing.T) {
			t.Parallel()
			fm := NewForwardManager(context.Background(), address, model.Localhost, "0.0.0.0", nil, "")
			if _, err := fm.SSHPort(); err == nil {
				t.Fatalf("SSHPort accepted %q", address)
			}
		})
	}

	fm := NewForwardManager(context.Background(), net.JoinHostPort(sshtransport.LoopbackHost, "2222"), model.Localhost, "0.0.0.0", nil, "")
	port, err := fm.SSHPort()
	if err != nil || port != 2222 {
		t.Fatalf("SSHPort() = %d, %v; want 2222, nil", port, err)
	}
	fm = NewForwardManager(context.Background(), "[::1]:2222", model.Localhost, "0.0.0.0", nil, "")
	if port, err := fm.SSHPort(); err != nil || port != 2222 {
		t.Fatalf("IPv6 SSHPort() = %d, %v; want 2222, nil", port, err)
	}
}

func TestForwardManagerCanonicalizesCompatibleRequestedAddresses(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"localhost:0": "127.0.0.1:0",
		"0.0.0.0:0":   "127.0.0.1:0",
		"[::]:0":      "[::1]:0",
	}
	for requested, want := range tests {
		fm := NewForwardManager(context.Background(), requested, model.Localhost, "0.0.0.0", nil, "")
		if fm.sshAddrErr != nil {
			t.Fatalf("NewForwardManager(%q) error = %v", requested, fm.sshAddrErr)
		}
		if fm.sshAddr != want {
			t.Fatalf("NewForwardManager(%q) address = %q, want %q", requested, fm.sshAddr, want)
		}
	}
}

func (t *testHTTPHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	oktetoLog.Println(fmt.Sprintf("message %s", t.message))
	_, err := w.Write([]byte(t.message))
	if err != nil {
		oktetoLog.Infof("error writing message %s: %s", t.message, err)
	}
}

func (*testSSHHandler) listenAndServe(address string) {
	forwardHandler := &ssh.ForwardedTCPHandler{}
	server := &ssh.Server{
		Addr: address,
		ChannelHandlers: map[string]ssh.ChannelHandler{
			"direct-tcpip": ssh.DirectTCPIPHandler,
			"session":      ssh.DefaultSessionHandler,
		},
		LocalPortForwardingCallback: ssh.LocalPortForwardingCallback(func(ctx ssh.Context, dhost string, dport uint32) bool {
			oktetoLog.Println("Accepted forward", dhost, dport)
			return true
		}),
		ReversePortForwardingCallback: ssh.ReversePortForwardingCallback(func(ctx ssh.Context, host string, port uint32) bool {
			oktetoLog.Println("attempt to bind", host, port, "granted")
			return true
		}),
		RequestHandlers: map[string]ssh.RequestHandler{
			"tcpip-forward":        forwardHandler.HandleSSHRequest,
			"cancel-tcpip-forward": forwardHandler.HandleSSHRequest,
		},
	}

	if err := server.ListenAndServe(); err != nil {
		oktetoLog.Fatalf("%s", err.Error())
	}
}

func TestForward(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	sshPort, err := model.GetAvailablePort(model.Localhost)
	if err != nil {
		t.Fatal(err)
	}

	sshAddr := net.JoinHostPort(sshtransport.LoopbackHost, strconv.Itoa(sshPort))
	ssh := testSSHHandler{}
	go ssh.listenAndServe(sshAddr)
	fm := NewForwardManager(ctx, sshAddr, model.Localhost, "0.0.0.0", nil, "")

	if err := startServers(fm); err != nil {
		t.Fatal(err)
	}

	if err := fm.Start("", ""); err != nil {
		t.Fatal(err)
	}

	if err := fm.waitForwardsConnected(); err != nil {
		t.Fatal(err)
	}

	oktetoLog.Info("forwards connected")

	if err := callForwards(fm); err != nil {
		t.Error(err)
	}

	cancel()
	fm.Stop()
	if err := fm.waitForwardsDisconnected(); err != nil {
		t.Error(err)
	}

	if !fm.pool.stopped.Load() {
		t.Error("pool is not stopped")
	}
}

func TestReverse(t *testing.T) {
	ctx := context.Background()
	sshPort, err := model.GetAvailablePort(model.Localhost)
	if err != nil {
		t.Fatal(err)
	}

	sshAddr := net.JoinHostPort(sshtransport.LoopbackHost, strconv.Itoa(sshPort))
	ssh := testSSHHandler{}
	go ssh.listenAndServe(sshAddr)
	fm := NewForwardManager(ctx, sshAddr, model.Localhost, "0.0.0.0", nil, "")

	if err := connectReverseForwards(fm); err != nil {
		t.Fatal(err)
	}

	if err := fm.Start("", ""); err != nil {
		t.Fatal(err)
	}

	if err := checkReverseForwardsConnected(fm); err != nil {
		t.Fatal(err)
	}

	if err := callReverseForwards(fm); err != nil {
		t.Error(err)
	}

}

func startServers(fm *ForwardManager) error {
	for i := 0; i < 1; i++ {
		local, err := model.GetAvailablePort(model.Localhost)
		if err != nil {
			return err
		}

		remote, err := model.GetAvailablePort(model.Localhost)
		if err != nil {
			return err
		}

		if err := fm.Add(forwardModel.Forward{Local: local, Remote: remote}); err != nil {
			return err
		}

		go func() {
			handler := &testHTTPHandler{message: fmt.Sprintf("%d", remote)}
			server := &http.Server{
				Addr:              net.JoinHostPort("", strconv.Itoa(remote)),
				Handler:           handler,
				ReadHeaderTimeout: 3 * time.Second,
			}

			err = server.ListenAndServe()
			if err != nil {
				oktetoLog.Infof("reverse server %d failed: %s", local, err.Error())
			}
		}()
	}

	return nil
}

func connectReverseForwards(fm *ForwardManager) error {
	for i := 0; i < 1; i++ {
		local, err := model.GetAvailablePort(model.Localhost)
		if err != nil {
			return err
		}

		remote, err := model.GetAvailablePort(model.Localhost)
		if err != nil {
			return err
		}

		if err := fm.AddReverse(model.Reverse{Local: local, Remote: remote}); err != nil {
			return err
		}

		go func() {
			handler := &testHTTPHandler{message: fmt.Sprintf("%d", local)}
			server := &http.Server{
				Addr:              net.JoinHostPort("", strconv.Itoa(local)),
				Handler:           handler,
				ReadHeaderTimeout: 3 * time.Second,
			}

			err = server.ListenAndServe()
			if err != nil {
				oktetoLog.Infof("reverse server %d failed: %s", local, err.Error())
			}
		}()
	}

	return nil
}

func checkReverseForwardsConnected(fm *ForwardManager) error {
	tk := time.NewTicker(10 * time.Millisecond)
	var connected bool
	for i := 0; i < 100; i++ {
		connected = true
		for _, r := range fm.reverses {
			connected = connected && r.connected()
		}

		if connected {
			break
		}
		<-tk.C
	}

	if !connected {
		return fmt.Errorf("reverse forwards not connected")
	}

	return nil
}

func callForwards(fm *ForwardManager) error {
	for _, f := range fm.forwards {
		localPort := getPort(f.localAddress)
		r, err := http.Get(fmt.Sprintf("http://localhost:%s", localPort))
		if err != nil {
			return fmt.Errorf("%s failed: %w", f.String(), err)
		}

		if r.StatusCode != 200 {
			return fmt.Errorf("%s bad response: %d | %s ", f.String(), r.StatusCode, r.Status)
		}

		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return fmt.Errorf("%s failed to read response: %w", f.String(), err)
		}

		got := string(body)
		remotePort := getPort(f.remoteAddress)
		if got != remotePort {
			return fmt.Errorf("%s got: %s, expected: %s", f.String(), got, remotePort)
		}
	}

	return nil
}

func callReverseForwards(fm *ForwardManager) error {
	for _, r := range fm.reverses {
		remotePort := getPort(r.remoteAddress)
		resp, err := http.Get(fmt.Sprintf("http://localhost:%s", remotePort))
		if err != nil {
			return fmt.Errorf("%s failed: %w", r.String(), err)
		}

		if resp.StatusCode != 200 {
			return fmt.Errorf("%s bad response: %d | %s ", r.String(), resp.StatusCode, resp.Status)
		}

		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("%s failed to read response: %w", r.String(), err)
		}

		got := string(body)
		localPort := getPort(r.localAddress)
		expected := localPort
		if got != expected {
			return fmt.Errorf("%s got: %s, expected: %s", r.String(), got, expected)
		}
	}

	return nil
}

func getPort(address string) string {
	parts := strings.Split(address, ":")
	return parts[1]
}

func (fm *ForwardManager) waitForwardsConnected() error {
	connectTimeout := 120 * time.Second
	tk := time.NewTicker(500 * time.Millisecond)
	start := time.Now()
	var connected bool

	for {
		elapsed := time.Since(start)
		if elapsed > connectTimeout {
			return fmt.Errorf("forwards not connected after %s", connectTimeout)
		}

		connected = true
		for _, f := range fm.forwards {
			connected = connected && f.connected()
		}

		if connected {
			return nil
		}
		<-tk.C
	}
}

func (fm *ForwardManager) waitForwardsDisconnected() error {
	connectTimeout := 120 * time.Second
	tk := time.NewTicker(500 * time.Millisecond)
	start := time.Now()

	for {
		elapsed := time.Since(start)
		if elapsed > connectTimeout {
			return fmt.Errorf("forwards not disconnected after %s", connectTimeout)
		}

		disconnected := true
		for _, f := range fm.forwards {
			if f.connected() {
				oktetoLog.Infof("%s is still connected", f)
				disconnected = false
			}
		}

		if disconnected {
			return nil
		}

		<-tk.C
	}
}

func TestAdd(t *testing.T) {

	pf := NewForwardManager(context.Background(), "0.0.0.0:22000", "0.0.0.0", "0.0.0.0", nil, "")
	if err := pf.Add(forwardModel.Forward{Local: 10010, Remote: 1010}); err != nil {
		t.Fatal(err)
	}

	if err := pf.Add(forwardModel.Forward{Local: 10011, Remote: 1011}); err != nil {
		t.Fatal(err)
	}

	if err := pf.Add(forwardModel.Forward{Local: 10010, Remote: 1011}); err == nil {
		t.Fatal("duplicated local port didn't return an error")
	}

	if err := pf.Add(forwardModel.Forward{Local: 10012, Remote: 15123, Service: true, ServiceName: "svc"}); err != nil {
		t.Fatal(err)
	}

	if pf.forwards[10012].remoteAddress != "svc:15123" {
		t.Fatalf("expected 'svc:15123', got '%s'", pf.forwards[1012].remoteAddress)
	}
}
