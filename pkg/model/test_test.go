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

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v3"
)

func TestHostUnmarshalYAML(t *testing.T) {
	t.Setenv("IP", "192.179.1.1")
	tests := []struct {
		expectedError error
		expectedHost  Host
		name          string
		bytes         []byte
	}{
		{
			name:  "ipv4",
			bytes: []byte("main.example.com:192.168.1.100"),
			expectedHost: Host{
				Hostname: "main.example.com",
				IP:       "192.168.1.100",
			},
		},
		{
			name:  "ipv4 with expansion",
			bytes: []byte("main.example.com:${IP}"),
			expectedHost: Host{
				Hostname: "main.example.com",
				IP:       "192.179.1.1",
			},
		},
		{
			name:          "ipv4 with expansion to non-existent env var",
			bytes:         []byte("main.example.com:${NON_EXISTENT}"),
			expectedError: ErrInvalidIp,
		},
		{
			name:  "ipv6",
			bytes: []byte("main.example.com:::1"),
			expectedHost: Host{
				Hostname: "main.example.com",
				IP:       "::1",
			},
		},
		{
			name:          "malformed - no ip",
			bytes:         []byte("main.example.com:"),
			expectedError: ErrInvalidHostName,
		},
		{
			name:          "malformed - no hostname",
			bytes:         []byte(":192.168.1.1"),
			expectedError: ErrInvalidHostName,
		},
		{
			name:          "malformed - no separator",
			bytes:         []byte("main.example.com192.168.1.1"),
			expectedError: ErrHostMalformed,
		},
		{
			name:          "extended without ip",
			bytes:         []byte("hostname: main.example.com"),
			expectedError: ErrInvalidIp,
		},
		{
			name:          "extended without hostname",
			bytes:         []byte("ip: 192.168.1.1"),
			expectedError: ErrInvalidHostName,
		},
		{
			name: "extended",
			bytes: []byte(`hostname: main.example.com
ip: 192.168.1.1`),
			expectedHost: Host{
				Hostname: "main.example.com",
				IP:       "192.168.1.1",
			},
		},
		{
			name: "extended with expansion",
			bytes: []byte(`hostname: main.example.com
ip: ${IP}`),
			expectedHost: Host{
				Hostname: "main.example.com",
				IP:       "192.179.1.1",
			},
		},
		{
			name: "extended with expansion to non-existent env var",
			bytes: []byte(`hostname: main.example.com
ip: ${NON_EXISTENT}`),
			expectedError: ErrInvalidIp,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var host Host
			err := yaml.Unmarshal(tt.bytes, &host)
			if tt.expectedError != nil {
				assert.ErrorIs(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedHost, host)
			}
		})
	}
}
