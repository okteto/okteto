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
)

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
