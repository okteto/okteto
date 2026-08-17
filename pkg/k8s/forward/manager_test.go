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

package forward

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/model/forward"
	"k8s.io/client-go/tools/portforward"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type fakeDevPortForwarder struct {
	forward func() error
	ports   []portforward.ForwardedPort
	portErr error
}

func (f *fakeDevPortForwarder) ForwardPorts() error {
	return f.forward()
}

func (f *fakeDevPortForwarder) GetPorts() ([]portforward.ForwardedPort, error) {
	return f.ports, f.portErr
}

func newTestActive() *active {
	return &active{
		readyChan: make(chan struct{}),
		stopChan:  make(chan struct{}),
		doneChan:  make(chan struct{}),
	}
}

func TestAddAcceptsAutomaticPortWithoutPrebinding(t *testing.T) {
	pf := NewPortForwardManager(context.Background(), "127.0.0.1", nil, nil, "")
	if err := pf.Add(forward.Forward{Local: 0, Remote: 2222}); err != nil {
		t.Fatalf("automatic port was rejected: %v", err)
	}
	if got := pf.ports[0]; got.Local != 0 || got.Remote != 2222 {
		t.Fatalf("automatic port mapping = %+v, want 0:2222", got)
	}
	if err := pf.Add(forward.Forward{Local: 0, Remote: 2223}); err == nil {
		t.Fatal("duplicate automatic port was accepted")
	}
}

func TestStartDoesNotTreatFailureAsReadiness(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("listener bind failed")
	pf := NewPortForwardManager(context.Background(), "127.0.0.1", nil, nil, "")
	pf.buildDevForwarder = func(context.Context, string, string) (*active, devPortForwarder, error) {
		a := newTestActive()
		return a, &fakeDevPortForwarder{forward: func() error { return sentinel }}, nil
	}

	if err := pf.Start("pod", "namespace"); !errors.Is(err, sentinel) {
		t.Fatalf("Start() error = %v, want %v", err, sentinel)
	}
	if _, err := pf.ForwardedPorts(); err == nil {
		t.Fatal("ForwardedPorts succeeded after startup failure")
	}
}

func TestStartContextCancelsForwarderBeforeReadiness(t *testing.T) {
	t.Parallel()

	forwarding := make(chan struct{})
	pf := NewPortForwardManager(context.Background(), "127.0.0.1", nil, nil, "")
	pf.buildDevForwarder = func(ctx context.Context, _ string, _ string) (*active, devPortForwarder, error) {
		a := newTestActive()
		return a, &fakeDevPortForwarder{forward: func() error {
			close(forwarding)
			<-ctx.Done()
			return ctx.Err()
		}}, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- pf.StartContext(ctx, "pod", "namespace") }()
	<-forwarding
	cancel()

	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("StartContext() error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("StartContext did not stop after cancellation")
	}
	if _, err := pf.ForwardedPorts(); err == nil {
		t.Fatal("cancelled attempt remained active")
	}
}

func TestStartContextDisarmsStartupCancellationAfterReadiness(t *testing.T) {
	t.Parallel()

	pf := NewPortForwardManager(context.Background(), "127.0.0.1", nil, nil, "")
	var attemptCtx context.Context
	pf.buildDevForwarder = func(ctx context.Context, _ string, _ string) (*active, devPortForwarder, error) {
		attemptCtx = ctx
		a := newTestActive()
		return a, &fakeDevPortForwarder{
			ports: []portforward.ForwardedPort{{Local: 34567, Remote: 2222}},
			forward: func() error {
				close(a.readyChan)
				<-ctx.Done()
				return ctx.Err()
			},
		}, nil
	}
	startupCtx, cancelStartup := context.WithCancel(context.Background())
	if err := pf.StartContext(startupCtx, "pod", "namespace"); err != nil {
		t.Fatal(err)
	}
	cancelStartup()
	select {
	case <-attemptCtx.Done():
		t.Fatal("completed startup context remained attached to the live forward")
	case <-time.After(25 * time.Millisecond):
	}

	ports, err := pf.ForwardedPorts()
	if err != nil || len(ports) != 1 || ports[0].Local != 34567 {
		t.Fatalf("startup-context cancellation stopped live forward: ports=%+v error=%v", ports, err)
	}
	pf.Stop()
}

func TestContextDialerCancelsUpgradeRequest(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	requestStarted := make(chan struct{})
	dialer := &contextDialer{
		ctx: ctx,
		client: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			close(requestStarted)
			<-request.Context().Done()
			return nil, request.Context().Err()
		})},
		method: http.MethodPost,
		url:    &url.URL{Scheme: "http", Host: "127.0.0.1", Path: "/portforward"},
	}
	result := make(chan error, 1)
	go func() {
		_, _, err := dialer.Dial("portforward.k8s.io")
		result <- err
	}()
	<-requestStarted
	cancel()

	select {
	case err := <-result:
		if err == nil {
			t.Fatal("Dial succeeded after request context cancellation")
		}
	case <-time.After(time.Second):
		t.Fatal("SPDY upgrade request ignored context cancellation")
	}
}

func TestForwardedPortsRequiresLiveReadyAttempt(t *testing.T) {
	t.Parallel()

	a := newTestActive()
	fake := &fakeDevPortForwarder{
		ports: []portforward.ForwardedPort{{Local: 34567, Remote: 2222}},
		forward: func() error {
			close(a.readyChan)
			<-a.stopChan
			return nil
		},
	}
	pf := NewPortForwardManager(context.Background(), "127.0.0.1", nil, nil, "")
	pf.buildDevForwarder = func(context.Context, string, string) (*active, devPortForwarder, error) {
		return a, fake, nil
	}

	if _, err := pf.ForwardedPorts(); err == nil {
		t.Fatal("ForwardedPorts succeeded before Start")
	}
	if err := pf.Start("pod", "namespace"); err != nil {
		t.Fatal(err)
	}
	ports, err := pf.ForwardedPorts()
	if err != nil || !reflect.DeepEqual(ports, fake.ports) {
		t.Fatalf("ForwardedPorts() = %+v, %v; want %+v, nil", ports, err, fake.ports)
	}
	pf.Stop()
	if _, err := pf.ForwardedPorts(); err == nil {
		t.Fatal("ForwardedPorts succeeded after Stop")
	}
}

func TestForwardedPortsRejectsNotReadyAndPropagatesGetPortsError(t *testing.T) {
	t.Parallel()

	pf := NewPortForwardManager(context.Background(), "127.0.0.1", nil, nil, "")
	a := newTestActive()
	sentinel := errors.New("get ports failed")
	fake := &fakeDevPortForwarder{forward: func() error { return nil }, portErr: sentinel}
	pf.activeDev = a
	pf.activeDevPF = fake
	if _, err := pf.ForwardedPorts(); err == nil {
		t.Fatal("ForwardedPorts succeeded before readiness")
	}
	close(a.readyChan)
	if _, err := pf.ForwardedPorts(); !errors.Is(err, sentinel) {
		t.Fatalf("ForwardedPorts() error = %v, want %v", err, sentinel)
	}
}

func TestStartRejectsInvalidInjectedForwarderState(t *testing.T) {
	t.Parallel()

	for _, builder := range []func(context.Context, string, string) (*active, devPortForwarder, error){
		func(context.Context, string, string) (*active, devPortForwarder, error) {
			return nil, &fakeDevPortForwarder{}, nil
		},
		func(context.Context, string, string) (*active, devPortForwarder, error) {
			return newTestActive(), nil, nil
		},
		func(context.Context, string, string) (*active, devPortForwarder, error) {
			return &active{stopChan: make(chan struct{}), doneChan: make(chan struct{})}, &fakeDevPortForwarder{}, nil
		},
	} {
		pf := NewPortForwardManager(context.Background(), "127.0.0.1", nil, nil, "")
		pf.buildDevForwarder = builder
		if err := pf.Start("pod", "namespace"); err == nil {
			t.Fatal("Start accepted invalid injected forwarder state")
		}
	}
}

func TestStopDuringBuildPreventsForwarderLaunch(t *testing.T) {
	t.Parallel()

	building := make(chan struct{})
	release := make(chan struct{})
	forwardCalled := false
	pf := NewPortForwardManager(context.Background(), "127.0.0.1", nil, nil, "")
	pf.buildDevForwarder = func(context.Context, string, string) (*active, devPortForwarder, error) {
		close(building)
		<-release
		return newTestActive(), &fakeDevPortForwarder{forward: func() error {
			forwardCalled = true
			return nil
		}}, nil
	}

	result := make(chan error, 1)
	go func() { result <- pf.Start("pod", "namespace") }()
	<-building
	pf.Stop()
	close(release)
	if err := <-result; err == nil {
		t.Fatal("Start succeeded after Stop during construction")
	}
	if forwardCalled {
		t.Fatal("forwarder launched after Stop")
	}
}

func TestDelayedOldAttemptCannotClearNewAttempt(t *testing.T) {
	t.Parallel()

	firstRunning := make(chan struct{})
	releaseFirst := make(chan struct{})
	var mu sync.Mutex
	call := 0
	var second *active
	pf := NewPortForwardManager(context.Background(), "127.0.0.1", nil, nil, "")
	pf.stopTimeout = 10 * time.Millisecond
	pf.buildDevForwarder = func(context.Context, string, string) (*active, devPortForwarder, error) {
		mu.Lock()
		defer mu.Unlock()
		call++
		a := newTestActive()
		if call == 1 {
			return a, &fakeDevPortForwarder{forward: func() error {
				close(firstRunning)
				<-releaseFirst
				return errors.New("old attempt failed")
			}}, nil
		}
		second = a
		return a, &fakeDevPortForwarder{
			ports: []portforward.ForwardedPort{{Local: 34568, Remote: 2222}},
			forward: func() error {
				close(a.readyChan)
				<-a.stopChan
				return nil
			},
		}, nil
	}

	firstResult := make(chan error, 1)
	go func() { firstResult <- pf.Start("pod-one", "namespace") }()
	<-firstRunning
	pf.Stop()
	if err := pf.Start("pod-two", "namespace"); err != nil {
		t.Fatal(err)
	}
	close(releaseFirst)
	if err := <-firstResult; err == nil {
		t.Fatal("old attempt unexpectedly succeeded")
	}

	pf.mu.RLock()
	gotActive := pf.activeDev
	pf.mu.RUnlock()
	if gotActive != second {
		t.Fatal("old attempt cleared the active retry")
	}
	ports, err := pf.ForwardedPorts()
	if err != nil || len(ports) != 1 || ports[0].Local != 34568 {
		t.Fatalf("new attempt ports = %+v, %v", ports, err)
	}
	pf.Stop()
}

func TestAdd(t *testing.T) {

	pf := NewPortForwardManager(context.Background(), model.Localhost, nil, nil, "")
	if err := pf.Add(forward.Forward{Local: 10100, Remote: 1010}); err != nil {
		t.Fatal(err)
	}

	if err := pf.Add(forward.Forward{Local: 10110, Remote: 1011}); err != nil {
		t.Fatal(err)
	}

	if err := pf.Add(forward.Forward{Local: 10100, Remote: 1011}); err == nil {
		t.Fatal("duplicated local port didn't return an error")
	}

	if err := pf.Add(forward.Forward{Local: 10120, Remote: 0, Service: true, ServiceName: "svc"}); err != nil {
		t.Fatal(err)
	}

	if len(pf.ports) != 3 {
		t.Fatalf("expected 3 ports but got %d", len(pf.ports))
	}

	if _, ok := pf.services["svc"]; !ok {
		t.Errorf("service/svc wasn't added to list: %+v", pf.services)
	}
}

func TestStop(t *testing.T) {
	pf := NewPortForwardManager(context.Background(), model.Localhost, nil, nil, "")
	pf.activeDev = &active{
		readyChan: make(chan struct{}, 1),
		stopChan:  make(chan struct{}, 1),
	}

	pf.activeServices = map[string]*active{
		"svc": {
			readyChan: make(chan struct{}, 1),
			stopChan:  make(chan struct{}, 1),
		},
	}

	pf.Stop()
	if !pf.stopped {
		t.Error("pf wasn't marked as stopped")
	}

	if pf.activeDev != nil {
		t.Error("pf.activeDev wasn't set to nil")
	}

	if pf.activeServices != nil {
		t.Error("pf.activeServices wasn't to nil")
	}
}

func Test_active_stop(t *testing.T) {
	tests := []struct {
		stopChan chan struct{}
		name     string
		stop     bool
	}{
		{
			name: "nil-channel",
		},
		{
			name:     "channel",
			stopChan: make(chan struct{}, 1),
		},
		{
			name:     "stopped-channel",
			stopChan: make(chan struct{}, 1),
			stop:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &active{
				stopChan: tt.stopChan,
			}

			if tt.stop {
				a.stop()
			}

			a.stop()
		})
	}
}

func Test_active_finishIsIdempotent(t *testing.T) {
	a := newTestActive()
	want := errors.New("finished")
	a.finish(want)
	a.finish(errors.New("ignored"))
	if !errors.Is(a.error(), want) {
		t.Fatalf("active error = %v, want %v", a.error(), want)
	}
	select {
	case <-a.doneChan:
	default:
		t.Fatal("finish did not close done channel")
	}
}

func Test_getServicePorts(t *testing.T) {
	tests := []struct {
		name     string
		forwards map[int]forward.Forward
		expected []string
	}{
		{
			name: "services-with-port",
			forwards: map[int]forward.Forward{
				80:   {Local: 80, Remote: 8090},
				8080: {Local: 8080, Remote: 8090, ServiceName: "svc", Service: true},
				22:   {Local: 22000, Remote: 22},
			},
			expected: []string{"8080:8090"},
		},
		{
			name: "services-with-multiple-ports",
			forwards: map[int]forward.Forward{
				80:   {Local: 80, Remote: 8090},
				8080: {Local: 8080, Remote: 8090, ServiceName: "svc", Service: true},
				22:   {Local: 22000, Remote: 22},
				8089: {Local: 8089, Remote: 80890, ServiceName: "svc", Service: true},
			},
			expected: []string{"8080:8090", "8089:80890"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ports := getServicePorts("svc", tt.forwards)
			sort.Strings(ports)
			if !reflect.DeepEqual(ports, tt.expected) {
				t.Errorf("Expected: %+v, Got: %+v", tt.expected, ports)
			}
		})
	}
}
