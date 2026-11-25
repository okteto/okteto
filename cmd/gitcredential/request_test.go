// Copyright 2026 The Okteto Authors
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

package gitcredential

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadRequest(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected request
	}{
		{
			name:     "request from Git with useHttpPath",
			input:    "protocol=https\nhost=github.com\npath=acme/infra.git\n\n",
			expected: request{protocol: "https", host: "github.com", path: "acme/infra.git"},
		},
		{
			name:     "keys the helper doesn't need are ignored",
			input:    "capability[]=authtype\ncapability[]=state\nprotocol=https\nhost=github.com\npath=acme/infra.git\nwwwauth[]=Basic realm=\"GitHub\"\n\n",
			expected: request{protocol: "https", host: "github.com", path: "acme/infra.git"},
		},
		{
			name:     "request without a trailing blank line",
			input:    "protocol=https\nhost=github.com",
			expected: request{protocol: "https", host: "github.com"},
		},
		{
			name:     "a blank line ends the request",
			input:    "protocol=https\nhost=github.com\n\npath=acme/infra.git\n",
			expected: request{protocol: "https", host: "github.com"},
		},
		{
			name:     "CRLF line endings",
			input:    "protocol=https\r\nhost=github.com\r\npath=acme/infra.git\r\n\r\n",
			expected: request{protocol: "https", host: "github.com", path: "acme/infra.git"},
		},
		{
			name:     "lines without a key are ignored",
			input:    "protocol=https\nnot a key\nhost=github.com\n\n",
			expected: request{protocol: "https", host: "github.com"},
		},
		{
			name:     "empty request",
			input:    "",
			expected: request{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := readRequest(strings.NewReader(tt.input))

			require.NoError(t, err)
			require.Equal(t, tt.expected, req)
		})
	}
}

func TestRequestIsGitHub(t *testing.T) {
	tests := []struct {
		name     string
		req      request
		expected bool
	}{
		{
			name:     "https on github.com",
			req:      request{protocol: "https", host: "github.com"},
			expected: true,
		},
		{
			name:     "host in upper case",
			req:      request{protocol: "https", host: "GitHub.com"},
			expected: true,
		},
		{
			name:     "plain http",
			req:      request{protocol: "http", host: "github.com"},
			expected: false,
		},
		{
			name:     "host with a port",
			req:      request{protocol: "https", host: "github.com:443"},
			expected: false,
		},
		{
			name:     "another host",
			req:      request{protocol: "https", host: "gitlab.com"},
			expected: false,
		},
		{
			name:     "GitHub subdomain",
			req:      request{protocol: "https", host: "api.github.com"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.req.isGitHub())
		})
	}
}

func TestRequestRepository(t *testing.T) {
	tests := []struct {
		expectedErr error
		name        string
		path        string
		expected    string
	}{
		{
			name:     "owner and repo",
			path:     "acme/infra",
			expected: "acme/infra",
		},
		{
			name:     "with the .git suffix",
			path:     "acme/infra.git",
			expected: "acme/infra",
		},
		{
			name:     "with slashes around",
			path:     "/acme/infra.git/",
			expected: "acme/infra",
		},
		{
			name:     "Git LFS path",
			path:     "acme/infra.git/info/lfs/objects/batch",
			expected: "acme/infra",
		},
		{
			name:        "no path",
			path:        "",
			expectedErr: errMissingPath,
		},
		{
			name:        "only the owner",
			path:        "acme",
			expectedErr: errMissingRepository,
		},
		{
			name:        "only the .git suffix as repo",
			path:        "acme/.git",
			expectedErr: errMissingRepository,
		},
		{
			name:        "empty segment after the owner",
			path:        "acme//infra",
			expectedErr: errMissingRepository,
		},
		{
			name:     "every character GitHub allows",
			path:     "acme-2/my_infra.v2.git",
			expected: "acme-2/my_infra.v2",
		},
		{
			name:        "terminal escape sequence decoded by Git",
			path:        "acme/infra\x1b[31m.git",
			expectedErr: errInvalidRepository,
		},
		{
			name:        "space in the owner",
			path:        "ac me/infra",
			expectedErr: errInvalidRepository,
		},
		{
			name:        "parent directory as repo",
			path:        "acme/..",
			expectedErr: errInvalidRepository,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository, err := request{path: tt.path}.repository()

			require.ErrorIs(t, err, tt.expectedErr)
			require.Equal(t, tt.expected, repository)
		})
	}
}
