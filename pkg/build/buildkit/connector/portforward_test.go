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
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/analytics"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log/io"
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
	require.Empty(t, pf.podName, "pod should be released so the retry can be reassigned")
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
