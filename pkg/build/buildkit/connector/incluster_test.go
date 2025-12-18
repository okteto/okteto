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

func TestInClusterConnector_Stop(t *testing.T) {
	tests := []struct {
		name          string
		initialPodIP  string
		expectedPodIP string
	}{
		{
			name:          "stop with podIP clears it",
			initialPodIP:  "10.0.0.1",
			expectedPodIP: "",
		},
		{
			name:          "stop without podIP does nothing",
			initialPodIP:  "",
			expectedPodIP: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ic := &InClusterConnector{
				podIP:  tt.initialPodIP,
				ioCtrl: io.NewIOController(),
			}

			// Should not panic
			require.NotPanics(t, func() {
				ic.Stop()
			})

			require.Equal(t, tt.expectedPodIP, ic.podIP)
		})
	}
}

func TestInClusterConnector_Stop_MultipleCallsSafe(t *testing.T) {
	ic := &InClusterConnector{
		podIP:  "10.0.0.1",
		ioCtrl: io.NewIOController(),
	}

	// First stop should work
	require.NotPanics(t, func() {
		ic.Stop()
	})
	require.Equal(t, "", ic.podIP)

	// Second stop should also not panic (idempotent)
	require.NotPanics(t, func() {
		ic.Stop()
	})

	// Third stop should still not panic
	require.NotPanics(t, func() {
		ic.Stop()
	})
}

func TestInClusterConnector_GetBuildkitClient_NoPodIP(t *testing.T) {
	ic := &InClusterConnector{
		podIP:  "",
		ioCtrl: io.NewIOController(),
	}

	_, err := ic.GetBuildkitClient(t.Context())
	require.Error(t, err)
	require.Contains(t, err.Error(), "no buildkit pod IP available")
}
