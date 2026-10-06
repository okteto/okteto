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
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/okteto/okteto/internal/test/client"
	"github.com/okteto/okteto/pkg/analytics"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/require"
)

// runWithDeadlockGuard runs fn on a separate goroutine and fails the test if it does not
// return within a timeout, which would indicate the process-wide deadlock this package
// used to hit when the port forward failed while Start() was holding the mutex.
func runWithDeadlockGuard(t *testing.T, fn func() error) error {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- errors.New("panic in guarded function")
			}
		}()
		done <- fn()
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(3 * time.Second):
		t.Fatal("deadlock: function did not return")
		return nil
	}
}

func TestPortForwarder_Stop(t *testing.T) {
	tests := []struct {
		name     string
		stopChan chan struct{}
		isActive bool
	}{
		{
			name:     "stop with open channel",
			stopChan: make(chan struct{}, 1),
			isActive: true,
		},
		{
			name:     "stop with nil channel",
			stopChan: nil,
			isActive: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pf := &PortForwarder{
				stopChan:  tt.stopChan,
				readyChan: make(chan struct{}, 1),
				localPort: 8080,
				ioCtrl:    io.NewIOController(),
				isActive:  tt.isActive,
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
		isActive:  true,
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

func TestPortForwarder_WaitUntilReady_UnblockedByPortForwardFailure(t *testing.T) {
	pf := &PortForwarder{
		stopChan:  make(chan struct{}, 1),
		readyChan: make(chan struct{}, 1),
		errChan:   make(chan error, 1),
		podName:   "buildkit-0",
		localPort: 8080,
		ioCtrl:    io.NewIOController(),
		metrics:   NewConnectorMetrics(analytics.ConnectorTypePortForward, "test-session", fakeConnectionTracker{}),
	}

	forwardErr := errors.New("error upgrading connection: connect: connection timed out")
	go pf.handlePortForwardError(forwardErr, pf.errChan)

	err := runWithDeadlockGuard(t, func() error {
		// Start() holds the mutex while waiting for the port forward to become ready
		pf.mu.Lock()
		defer pf.mu.Unlock()
		return pf.waitUntilPortForwardIsReady(context.Background())
	})

	require.EqualError(t, err, "port forward creation to BuildKit has failed")
	require.NotContains(t, err.Error(), "connection timed out", "raw cause must only go to the logs")
	var userErr oktetoErrors.UserError
	require.ErrorAs(t, err, &userErr)
	require.Contains(t, userErr.Hint, "--log-level=info")
	require.Eventually(t, func() bool {
		pf.mu.Lock()
		defer pf.mu.Unlock()
		return pf.podName == ""
	}, 3*time.Second, 10*time.Millisecond, "podName should be reset by the failure handler")
}

func TestPortForwarder_WaitUntilReady_ContextCancelledWhileHoldingLock(t *testing.T) {
	pf := &PortForwarder{
		stopChan:  make(chan struct{}, 1),
		readyChan: make(chan struct{}, 1),
		errChan:   make(chan error, 1),
		podName:   "buildkit-0",
		localPort: 8080,
		ioCtrl:    io.NewIOController(),
		metrics:   NewConnectorMetrics(analytics.ConnectorTypePortForward, "test-session", fakeConnectionTracker{}),
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := runWithDeadlockGuard(t, func() error {
		// Start() holds the mutex while waiting for the port forward to become ready
		pf.mu.Lock()
		defer pf.mu.Unlock()
		return pf.waitUntilPortForwardIsReady(ctx)
	})

	require.ErrorContains(t, err, "context cancelled while waiting for port forward")
	select {
	case _, ok := <-pf.stopChan:
		require.False(t, ok, "stopChan should be closed to terminate the forwarder")
	default:
		t.Error("stopChan should be closed but is still open")
	}
	require.False(t, pf.isActive, "port forward never became ready")
	require.Nil(t, pf.buildkitClient, "no client should be cached for a cancelled start")
	// the runner defers connector.Stop(); it must be safe after a cancelled Start
	require.NotPanics(t, pf.Stop)
}

func TestPortForwarder_HandlePortForwardError_AddressInUse(t *testing.T) {
	pf := &PortForwarder{
		errChan:   make(chan error, 1),
		podName:   "buildkit-0",
		localPort: 8080,
		ioCtrl:    io.NewIOController(),
	}

	pf.handlePortForwardError(errors.New("unable to listen on any of the requested ports"), pf.errChan)

	err := <-pf.errChan
	require.ErrorContains(t, err, "port 8080 is already in use")
	var userErr oktetoErrors.UserError
	require.ErrorAs(t, err, &userErr)
	require.NotEmpty(t, userErr.Hint)
	require.Equal(t, "buildkit-0", pf.podName, "pod assignment should be kept for a local port conflict")
}

func TestPortForwarder_WaitUntilReady_KeepsAddressInUseMessage(t *testing.T) {
	pf := &PortForwarder{
		stopChan:  make(chan struct{}, 1),
		readyChan: make(chan struct{}, 1),
		errChan:   make(chan error, 1),
		podName:   "buildkit-0",
		localPort: 8080,
		ioCtrl:    io.NewIOController(),
		metrics:   NewConnectorMetrics(analytics.ConnectorTypePortForward, "test-session", fakeConnectionTracker{}),
	}
	pf.handlePortForwardError(errors.New("unable to listen on any of the requested ports"), pf.errChan)

	err := pf.waitUntilPortForwardIsReady(context.Background())

	require.EqualError(t, err, "port 8080 is already in use", "specific message must not be replaced by the generic one")
	var userErr oktetoErrors.UserError
	require.ErrorAs(t, err, &userErr)
	require.Contains(t, userErr.Hint, "Check which process is using the port")
}

func TestPortForwarder_HandlePortForwardError_StaleAttemptDoesNotLeakIntoNewChannel(t *testing.T) {
	pf := &PortForwarder{
		stopChan:  make(chan struct{}, 1),
		readyChan: make(chan struct{}, 1),
		errChan:   make(chan error, 1),
		podName:   "buildkit-0",
		localPort: 8080,
		ioCtrl:    io.NewIOController(),
	}
	staleAttemptChan := pf.errChan

	// a retry re-establishes the port forward, replacing the connection channels
	pf.errChan = make(chan error, 1)

	// the old attempt's forwarder fails late, after the retry already started
	pf.handlePortForwardError(errors.New("lost connection to pod"), staleAttemptChan)

	require.Len(t, staleAttemptChan, 1, "error should land in the failed attempt's channel")
	require.Empty(t, pf.errChan, "new attempt's channel must not receive stale errors")
}

func TestPortForwarder_HandlePortForwardError_StaleAttemptKeepsNewConnection(t *testing.T) {
	pf := &PortForwarder{
		stopChan:  make(chan struct{}, 1),
		readyChan: make(chan struct{}, 1),
		errChan:   make(chan error, 1),
		podName:   "buildkit-0",
		localPort: 8080,
		ioCtrl:    io.NewIOController(),
	}
	staleAttemptChan := pf.errChan

	// a retry connects to another pod, replacing the connection channels
	pf.stopChan = make(chan struct{}, 1)
	pf.errChan = make(chan error, 1)
	pf.podName = "buildkit-1"
	pf.isActive = true

	// the old attempt's forwarder fails late, after the retry is already connected
	pf.handlePortForwardError(errors.New("lost connection to pod"), staleAttemptChan)

	require.Equal(t, "buildkit-1", pf.podName, "new attempt's pod must be kept")
	require.True(t, pf.isActive, "new attempt's connection must be kept")
	require.False(t, isClosed(pf.stopChan), "new attempt's forwarder must not be stopped")
}

func TestPortForwarder_HandlePortForwardError_CurrentAttemptReleasesPod(t *testing.T) {
	pf := &PortForwarder{
		stopChan:  make(chan struct{}, 1),
		readyChan: make(chan struct{}, 1),
		errChan:   make(chan error, 1),
		podName:   "buildkit-0",
		localPort: 8080,
		ioCtrl:    io.NewIOController(),
		isActive:  true,
	}

	// the connection drops after the port forward was ready
	pf.handlePortForwardError(errors.New("lost connection to pod"), pf.errChan)

	require.Empty(t, pf.podName, "pod should be released so the next start is reassigned")
	require.False(t, pf.isActive)
	require.True(t, isClosed(pf.stopChan), "stopChan should be closed to terminate the forwarder")
}

// isClosed reports whether ch is closed, without blocking
func isClosed(ch chan struct{}) bool {
	select {
	case _, ok := <-ch:
		return !ok
	default:
		return false
	}
}

// fakeBuildkitAPI assigns the given pods in order, one per request, or fails with err
type fakeBuildkitAPI struct {
	err   error
	pods  []string
	calls int
}

func (f *fakeBuildkitAPI) GetLeastLoadedBuildKitPod(context.Context, string) (*types.BuildKitPodResponse, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	if f.calls > len(f.pods) {
		return nil, fmt.Errorf("unexpected pod request #%d", f.calls)
	}
	return &types.BuildKitPodResponse{PodName: f.pods[f.calls-1]}, nil
}

// fakePortForward simulates the port forward to each pod: it fails synchronously with syncErr
// when set, becomes ready for the pods in reachable and fails asynchronously with forwardErr
// for the rest, like ForwardPorts does
type fakePortForward struct {
	pf         *PortForwarder
	reachable  map[string]bool
	syncErr    error
	forwardErr error
	failures   sync.WaitGroup
}

func (f *fakePortForward) start() error {
	if f.syncErr != nil {
		return f.syncErr
	}
	f.pf.stopChan = make(chan struct{}, 1)
	f.pf.readyChan = make(chan struct{}, 1)
	f.pf.errChan = make(chan error, 1)
	if f.reachable[f.pf.podName] {
		f.pf.readyChan <- struct{}{}
		return nil
	}
	errChan := f.pf.errChan
	f.failures.Add(1)
	go func() {
		defer f.failures.Done()
		f.pf.handlePortForwardError(f.forwardErr, errChan)
	}()
	return nil
}

// recordingConnectionTracker keeps the analytics events sent by the connector
type recordingConnectionTracker struct {
	events []*analytics.BuildkitConnectorMetadata
}

func (r *recordingConnectionTracker) TrackBuildkitConnection(m *analytics.BuildkitConnectorMetadata) {
	r.events = append(r.events, m)
}

// outcomes returns each tracked event as "success" or "failure: <reason>"
func (r *recordingConnectionTracker) outcomes() []string {
	outcomes := make([]string, 0, len(r.events))
	for _, e := range r.events {
		if e.Success {
			outcomes = append(outcomes, "success")
			continue
		}
		outcomes = append(outcomes, "failure: "+e.ErrReason)
	}
	return outcomes
}

// newPortForwarderWithFakes returns a port forwarder whose session is already assigned to buildkit-0
func newPortForwarderWithFakes(api *fakeBuildkitAPI, reachable map[string]bool, forwardErr error) (*PortForwarder, *fakePortForward, *recordingConnectionTracker) {
	tracker := &recordingConnectionTracker{}
	pf := &PortForwarder{
		oktetoClient: &client.FakeOktetoClient{BuildkitClient: api},
		podName:      "buildkit-0",
		localPort:    8080,
		maxWaitTime:  time.Minute,
		ioCtrl:       io.NewIOController(),
		metrics:      NewConnectorMetrics(analytics.ConnectorTypePortForward, "test-session", tracker),
	}
	fake := &fakePortForward{pf: pf, reachable: reachable, forwardErr: forwardErr}
	pf.portForward = fake.start
	return pf, fake, tracker
}

func TestPortForwarder_Start_ReassignsPodWhenAssignedPodIsGone(t *testing.T) {
	api := &fakeBuildkitAPI{pods: []string{"buildkit-1"}}
	pf, fake, tracker := newPortForwarderWithFakes(api, map[string]bool{"buildkit-1": true}, errors.New(`pods "buildkit-0" not found`))

	err := runWithDeadlockGuard(t, func() error { return pf.Start(context.Background()) })
	fake.failures.Wait()

	require.NoError(t, err)
	require.Equal(t, 1, api.calls, "a new pod should be requested once")
	require.Equal(t, "buildkit-1", pf.podName)
	require.True(t, pf.isActive, "the failure of the gone pod must not stop the new connection")
	require.Equal(t, []string{"failure: AssignedPodUnreachable", "success"}, tracker.outcomes(),
		"a recovered failure must not be reported as a port forward creation failure")
}

func TestPortForwarder_Start_ReusesAssignedPod(t *testing.T) {
	api := &fakeBuildkitAPI{}
	pf, _, tracker := newPortForwarderWithFakes(api, map[string]bool{"buildkit-0": true}, nil)

	err := runWithDeadlockGuard(t, func() error { return pf.Start(context.Background()) })

	require.NoError(t, err)
	require.Zero(t, api.calls, "the assigned pod should be reused")
	require.Equal(t, "buildkit-0", pf.podName)
	require.True(t, pf.isActive)
	require.Empty(t, tracker.outcomes())
}

func TestPortForwarder_Start_FailsWhenReassignedPodIsAlsoUnreachable(t *testing.T) {
	api := &fakeBuildkitAPI{pods: []string{"buildkit-1"}}
	pf, fake, tracker := newPortForwarderWithFakes(api, nil, errors.New("connection refused"))

	err := runWithDeadlockGuard(t, func() error { return pf.Start(context.Background()) })
	fake.failures.Wait()

	require.EqualError(t, err, "port forward creation to BuildKit has failed")
	require.Equal(t, 1, api.calls, "a new pod should be requested only once")
	require.Empty(t, pf.podName, "pod should be released so the next start is reassigned")
	require.False(t, pf.isActive)
	require.Equal(t, []string{"failure: AssignedPodUnreachable", "success", "failure: PortForwardCreation"}, tracker.outcomes())
}

func TestPortForwarder_Start_KeepsPortForwardErrorWhenNoNewPodCanBeAssigned(t *testing.T) {
	api := &fakeBuildkitAPI{err: errors.New("dial tcp: lookup okteto.example.com: no such host")}
	pf, fake, tracker := newPortForwarderWithFakes(api, nil, errors.New(`pods "buildkit-0" not found`))

	err := runWithDeadlockGuard(t, func() error { return pf.Start(context.Background()) })
	fake.failures.Wait()

	require.EqualError(t, err, "port forward creation to BuildKit has failed", "the raw backend error must only go to the logs")
	var userErr oktetoErrors.UserError
	require.ErrorAs(t, err, &userErr)
	require.Contains(t, userErr.Hint, "--log-level=info")
	require.Equal(t, 1, api.calls)
	require.Empty(t, pf.podName, "pod should be released so the next start is reassigned")
	require.Equal(t, []string{"failure: AssignedPodUnreachable", "failure: BackendInternalError"}, tracker.outcomes())
}

func TestPortForwarder_Start_DoesNotReassignPodWhenContextIsCancelled(t *testing.T) {
	api := &fakeBuildkitAPI{pods: []string{"buildkit-1"}}
	pf, fake, tracker := newPortForwarderWithFakes(api, map[string]bool{"buildkit-1": true}, nil)
	fake.syncErr = errors.New("failed to create SPDY round tripper")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := runWithDeadlockGuard(t, func() error { return pf.Start(ctx) })

	require.EqualError(t, err, "port forward creation to BuildKit has failed")
	require.Zero(t, api.calls, "a cancelled command must not request a new pod")
	require.Equal(t, "buildkit-0", pf.podName)
	require.Equal(t, []string{"failure: PortForwardCreation"}, tracker.outcomes())
}

func TestPortForwarder_Start_DoesNotReassignPodOnLocalPortConflict(t *testing.T) {
	api := &fakeBuildkitAPI{}
	pf, fake, tracker := newPortForwarderWithFakes(api, nil, errors.New("unable to listen on any of the requested ports"))

	err := runWithDeadlockGuard(t, func() error { return pf.Start(context.Background()) })
	fake.failures.Wait()

	require.EqualError(t, err, "port 8080 is already in use")
	require.Zero(t, api.calls, "another pod does not solve a local port conflict")
	require.Equal(t, "buildkit-0", pf.podName)
	require.Equal(t, []string{"failure: PortForwardCreation"}, tracker.outcomes())
}

func TestPortForwarder_GetWaiter(t *testing.T) {
	waiter := NewBuildkitClientWaiter(io.NewIOController())

	require.NotNil(t, waiter)
	require.NotNil(t, waiter.logger)
}

func TestPortForwarder_GetWaiter_Configuration(t *testing.T) {
	waiter := NewBuildkitClientWaiter(io.NewIOController())

	require.NotNil(t, waiter)
	require.NotNil(t, waiter.logger)
}
