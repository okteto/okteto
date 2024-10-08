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

package buildkit

import (
	"context"
	"encoding/base64"
	"net/url"
	"os"
	"testing"

	"github.com/moby/buildkit/client"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/stretchr/testify/assert"
)

// urlParser defines an interface for parsing URLs
type fakeURLParser struct {
	url *url.URL
	err error
}

// Parse parses a URL
func (p *fakeURLParser) Parse(rawurl string) (*url.URL, error) {
	return p.url, p.err
}

// certDecoder defines an interface for decoding certificates
type fakeCertDecoder struct {
	err  error
	data []byte
}

// DecodeString decodes a base64 string
func (b *fakeCertDecoder) DecodeString(s string) ([]byte, error) {
	return b.data, b.err
}

// fileWriter defines an interface for writing files
type fakeFileWriter struct {
	err error
}

// WriteFile writes a file
func (f *fakeFileWriter) WriteFile(filename string, data []byte, perm os.FileMode) error {
	return f.err
}

// mockBuildkitClientCreator is a mock implementation of the buildkitClientCreator interface.
type mockBuildkitClientCreator struct {
	client *client.Client
	err    error
}

// new creates a new BuildKit client
func (m *mockBuildkitClientCreator) New(ctx context.Context, address string, opts ...client.ClientOpt) (*client.Client, error) {
	return m.client, m.err
}

// TestBuildkitClientFactory_GetBuildkitClient tests the GetBuildkitClient method.
func TestBuildkitClientFactory_GetBuildkitClient(t *testing.T) {
	tests := []struct {
		urlParser       urlParser
		certDecoder     certDecoder
		fileWriter      fileWriter
		clientCreator   buildkitClientCreator
		name            string
		cert            string
		token           string
		builder         string
		certificatePath string
		expectedError   bool
	}{
		{
			name:            "Success with cert and token",
			cert:            base64.StdEncoding.EncodeToString([]byte("testcert")),
			token:           "testtoken",
			builder:         "https://buildkit.example.com",
			certificatePath: "/tmp/cert.pem",
			urlParser: &fakeURLParser{
				url: &url.URL{
					Host: "buildkit.example.com",
				},
				err: nil,
			},
			certDecoder: &fakeCertDecoder{
				data: []byte("testcert"),
				err:  nil,
			},
			fileWriter: &fakeFileWriter{
				err: nil,
			},
			clientCreator: &mockBuildkitClientCreator{
				client: &client.Client{},
				err:    nil,
			},
			expectedError: false,
		},
		{
			name:            "Error decoding certificate",
			cert:            "invalidbase64",
			token:           "testtoken",
			builder:         "https://buildkit.example.com",
			certificatePath: "/tmp/cert.pem",
			urlParser: &fakeURLParser{
				url: &url.URL{
					Host: "buildkit.example.com",
				},
				err: nil,
			},
			certDecoder: &fakeCertDecoder{
				data: []byte("testcert"),
				err:  assert.AnError,
			},
			fileWriter: &fakeFileWriter{
				err: nil,
			},
			clientCreator: &mockBuildkitClientCreator{
				client: &client.Client{},
				err:    nil,
			},
			expectedError: true,
		},
		{
			name:            "Error writing certificate file",
			cert:            base64.StdEncoding.EncodeToString([]byte("testcert")),
			token:           "testtoken",
			builder:         "https://buildkit.example.com",
			certificatePath: "/tmp/cert.pem",
			urlParser: &fakeURLParser{
				url: &url.URL{
					Host: "buildkit.example.com",
				},
				err: nil,
			},
			certDecoder: &fakeCertDecoder{
				data: []byte("testcert"),
				err:  nil,
			},
			fileWriter: &fakeFileWriter{
				err: assert.AnError,
			},
			clientCreator: &mockBuildkitClientCreator{
				client: &client.Client{},
				err:    nil,
			},
			expectedError: true,
		},
		{
			name:            "Error parsing builder URL",
			cert:            base64.StdEncoding.EncodeToString([]byte("testcert")),
			token:           "testtoken",
			builder:         "://invalid-url",
			certificatePath: "/tmp/cert.pem",
			urlParser: &fakeURLParser{
				url: nil,
				err: assert.AnError,
			},
			certDecoder: &fakeCertDecoder{
				data: []byte("testcert"),
				err:  nil,
			},
			fileWriter: &fakeFileWriter{
				err: nil,
			},
			clientCreator: &mockBuildkitClientCreator{
				client: &client.Client{},
				err:    nil,
			},
			expectedError: true,
		},
		{
			name:            "Error creating BuildKit client",
			cert:            base64.StdEncoding.EncodeToString([]byte("testcert")),
			token:           "testtoken",
			builder:         "https://buildkit.example.com",
			certificatePath: "/tmp/cert.pem",
			urlParser: &fakeURLParser{
				url: &url.URL{
					Host: "buildkit.example.com",
				},
				err: nil,
			},
			certDecoder: &fakeCertDecoder{
				data: []byte("testcert"),
				err:  nil,
			},
			fileWriter: &fakeFileWriter{
				err: nil,
			},
			clientCreator: &mockBuildkitClientCreator{
				client: nil,
				err:    assert.AnError,
			},
			expectedError: true,
		},
		{
			name:    "Success without cert and token",
			cert:    "",
			token:   "",
			builder: "unix:///var/run/buildkit/buildkitd.sock",
			urlParser: &fakeURLParser{
				url: &url.URL{
					Host: "buildkit.example.com",
				},
				err: nil,
			},
			clientCreator: &mockBuildkitClientCreator{
				client: &client.Client{},
				err:    nil,
			},
			expectedError: false,
		},
		{
			name:    "Error creating BuildKit client without cert",
			cert:    "",
			token:   "",
			builder: "unix:///var/run/buildkit/buildkitd.sock",
			clientCreator: &mockBuildkitClientCreator{
				client: nil,
				err:    assert.AnError,
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			bcf := &ClientFactory{
				cert:                  tt.cert,
				token:                 tt.token,
				builder:               tt.builder,
				certificatePath:       tt.certificatePath,
				urlParser:             tt.urlParser,
				certDecoder:           tt.certDecoder,
				fileWriter:            tt.fileWriter,
				buildkitClientCreator: tt.clientCreator,
				logger:                io.NewIOController(),
			}

			ctx := context.Background()
			client, err := bcf.GetBuildkitClient(ctx)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
			}
		})
	}
}
