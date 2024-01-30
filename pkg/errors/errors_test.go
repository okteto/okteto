// Copyright 2024 The Okteto Authors
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

package errors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsTransient(t *testing.T) {
	tests := []struct {
		err      error
		name     string
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "non transient error",
			err:      assert.AnError,
			expected: false,
		},
		{
			name:     "operation time out",
			err:      errors.New("operation time out"),
			expected: true,
		},
		{
			name:     "operation timed out",
			err:      errors.New("operation timed out"),
			expected: true,
		},
		{
			name:     "i/o timeout",
			err:      errors.New("i/o timeout"),
			expected: true,
		},
		{
			name:     "unknown (get events)",
			err:      errors.New("unknown (get events)"),
			expected: true,
		},
		{
			name:     "Client.Timeout exceeded while awaiting headers",
			err:      errors.New("Client.Timeout exceeded while awaiting headers"),
			expected: true,
		},
		{
			name:     "can't assign requested address",
			err:      errors.New("can't assign requested address"),
			expected: true,
		},
		{
			name:     "command exited without exit status or exit signal",
			err:      errors.New("command exited without exit status or exit signal"),
			expected: true,
		},
		{
			name:     "connection refused",
			err:      errors.New("connection refused"),
			expected: true,
		},
		{
			name:     "connection reset by peer",
			err:      errors.New("connection reset by peer"),
			expected: true,
		},
		{
			name:     "client connection lost",
			err:      errors.New("client connection lost"),
			expected: true,
		},
		{
			name:     "nodename nor servname provided, or not known",
			err:      errors.New("nodename nor servname provided, or not known"),
			expected: true,
		},
		{
			name:     "no route to host",
			err:      errors.New("no route to host"),
			expected: true,
		},
		{
			name:     "unexpected EOF",
			err:      errors.New("unexpected EOF"),
			expected: true,
		},
		{
			name:     "TLS handshake timeout",
			err:      errors.New("TLS handshake timeout"),
			expected: true,
		},
		{
			name:     "in the time allotted",
			err:      errors.New("in the time allotted"),
			expected: true,
		},
		{
			name:     "broken pipe",
			err:      errors.New("broken pipe"),
			expected: true,
		},
		{
			name:     "No connection could be made",
			err:      errors.New("No connection could be made"),
			expected: true,
		},
		{
			name:     "operation was canceled",
			err:      errors.New("operation was canceled"),
			expected: true,
		},
		{
			name:     "network is unreachable",
			err:      errors.New("network is unreachable"),
			expected: true,
		},
		{
			name:     "development container has been removed",
			err:      errors.New("development container has been removed"),
			expected: true,
		},
		{
			name:     "unexpected packet in response to channel open",
			err:      errors.New("unexpected packet in response to channel open"),
			expected: true,
		},
		{
			name:     "closing remote connection: EOF",
			err:      errors.New("closing remote connection: EOF"),
			expected: true,
		},
		{
			name:     "request for pseudo terminal failed: eof",
			err:      errors.New("request for pseudo terminal failed: eof"),
			expected: true,
		},
		{
			name:     "unable to upgrade connection",
			err:      errors.New("unable to upgrade connection"),
			expected: true,
		},
		{
			name:     "command execution failed: eof",
			err:      errors.New("command execution failed: eof"),
			expected: true,
		},
		{
			name:     "syncthing local=false didn't respond after",
			err:      errors.New("syncthing local=false didn't respond after 1m0s"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsTransient(tt.err)
			assert.Equal(t, tt.expected, got)
		})
	}
}
