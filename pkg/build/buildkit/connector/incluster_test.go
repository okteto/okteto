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

func TestInClusterConnector_GetBuildkitClient_NoPodIP(t *testing.T) {
	ic := &InClusterConnector{
		podIP:  "",
		ioCtrl: io.NewIOController(),
	}

	_, err := ic.GetBuildkitClient(t.Context())
	require.Error(t, err)
	require.Contains(t, err.Error(), "no buildkit pod IP available")
}

func TestBuildkitAddress(t *testing.T) {
	tests := []struct {
		name     string
		podIP    string
		expected string
	}{
		{name: "IPv4 pod", podIP: "10.42.15.211", expected: "tcp://10.42.15.211:1234"},
		{name: "IPv6 pod", podIP: "fd00:10:42:f::184c", expected: "tcp://[fd00:10:42:f::184c]:1234"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, buildkitAddress(tt.podIP))
		})
	}
}
