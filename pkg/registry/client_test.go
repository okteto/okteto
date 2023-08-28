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

package registry

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	containerv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoHttp "github.com/okteto/okteto/pkg/http"
	"github.com/stretchr/testify/assert"
)

type fakeTLSConn struct {
	handshake    bool
	handshakeErr error
}

func (c *fakeTLSConn) Handshake() error {
	c.handshake = true
	return c.handshakeErr
}

func (*fakeTLSConn) Close() error { return fmt.Errorf("Close() unimplemented") }

func (*fakeTLSConn) LocalAddr() net.Addr { return nil }

func (*fakeTLSConn) RemoteAddr() net.Addr { return nil }

func (*fakeTLSConn) Read([]byte) (int, error) { return 0, fmt.Errorf("Read() unimplemented") }

func (*fakeTLSConn) Write([]byte) (int, error) { return 0, fmt.Errorf("Write() unimplemented") }

func (*fakeTLSConn) SetDeadline(time.Time) error { return fmt.Errorf("SetDeadline() unimplemented") }

func (*fakeTLSConn) SetReadDeadline(time.Time) error {
	return fmt.Errorf("SetReadDeadline() unimplemented")
}

func (*fakeTLSConn) SetWriteDeadline(time.Time) error {
	return fmt.Errorf("SetWriteDeadline() unimplemented")
}

type fakeTLSDialArgs struct {
	network string
	addr    string
	config  *tls.Config
}

type fakeTLSDial struct {
	requests []fakeTLSDialArgs
	err      error
	mtx      sync.RWMutex
}

func (t *fakeTLSDial) tlsDial(network string, addr string, config *tls.Config) (oktetoHttp.TLSConn, error) {
	t.mtx.Lock()
	t.requests = append(t.requests, fakeTLSDialArgs{network: network, addr: addr, config: config})
	t.mtx.Unlock()
	return &fakeTLSConn{}, t.err
}

// FakeClient has everything needed to set up a test faking API calls
type fakeClient struct {
	GetImageDigest    getDigest
	GetConfig         getConfig
	MockGetDescriptor mockGetDescriptor
	MockWrite         mockWrite
	HasPushAcces      hasPushAccess
}

// GetDigest has everything needed to mock a getDigest API call
type getDigest struct {
	Result string
	Err    error
}

// GetConfig has everything needed to mock a getConfig API call
type getConfig struct {
	Result *containerv1.ConfigFile
	Err    error
}

type mockGetDescriptor struct {
	Result *remote.Descriptor
	Err    error
}

type mockWrite struct {
	Err error
}

type hasPushAccess struct {
	Result bool
	Err    error
}

func (fc fakeClient) GetDigest(_ string) (string, error) {
	return fc.GetImageDigest.Result, fc.GetImageDigest.Err
}

func (fc fakeClient) GetImageConfig(_ string) (*containerv1.ConfigFile, error) {
	return fc.GetConfig.Result, fc.GetConfig.Err
}

func (fc fakeClient) HasPushAccess(_ string) (bool, error) {
	return fc.HasPushAcces.Result, fc.HasPushAcces.Err
}

func (fc fakeClient) GetDescriptor(_ string) (*remote.Descriptor, error) {
	return fc.MockGetDescriptor.Result, fc.MockGetDescriptor.Err
}

func (fc fakeClient) Write(_ name.Reference, _ containerv1.Image) error {
	return fc.MockWrite.Err
}

type fakeClientConfig struct {
	registryURL                 string
	userID                      string
	token                       string
	isInsecure                  bool
	cert                        *x509.Certificate
	serverName                  string
	contextName                 string
	externalRegistryCredentials [2]string
}

func (f fakeClientConfig) GetRegistryURL() string                            { return f.registryURL }
func (f fakeClientConfig) GetUserID() string                                 { return f.userID }
func (f fakeClientConfig) GetToken() string                                  { return f.token }
func (f fakeClientConfig) IsInsecureSkipTLSVerifyPolicy() bool               { return f.isInsecure }
func (f fakeClientConfig) GetContextCertificate() (*x509.Certificate, error) { return f.cert, nil }
func (f fakeClientConfig) GetServerNameOverride() string                     { return f.serverName }
func (f fakeClientConfig) GetContextName() string                            { return f.contextName }
func (f fakeClientConfig) GetExternalRegistryCredentials(_ string) (string, string, error) {
	return f.externalRegistryCredentials[0], f.externalRegistryCredentials[1], nil
}

func TestGetDigest(t *testing.T) {
	unautorizedErr := &transport.Error{
		Errors: []transport.Diagnostic{
			{
				Code: transport.UnauthorizedErrorCode,
			},
		},
	}

	type input struct {
		config fakeClientConfig
		image  string
	}
	type getConfig struct {
		descriptor *remote.Descriptor
		err        error
	}
	type expected struct {
		image string
		err   error
	}
	var tests = []struct {
		name      string
		input     input
		getConfig getConfig
		expected  expected
	}{
		{
			name: "no error",
			input: input{
				config: fakeClientConfig{
					cert: &x509.Certificate{},
				},
				image: "okteto/test:latest",
			},
			getConfig: getConfig{
				descriptor: &remote.Descriptor{
					Descriptor: containerv1.Descriptor{
						Digest: containerv1.Hash{
							Hex: "testtestest",
						},
					},
				},
				err: nil,
			},
			expected: expected{
				image: ":testtestest",
				err:   nil,
			},
		},
		{
			name: "unauthorised error",
			input: input{
				config: fakeClientConfig{
					cert: &x509.Certificate{},
				},
				image: "okteto/test:latest",
			},
			getConfig: getConfig{
				descriptor: nil,
				err:        unautorizedErr,
			},
			expected: expected{
				image: "",
				err:   unautorizedErr,
			},
		},
		{
			name: "unauthorised error",
			input: input{
				config: fakeClientConfig{
					cert: &x509.Certificate{},
				},
				image: "okteto/test:latest",
			},
			getConfig: getConfig{
				descriptor: nil,
				err: &transport.Error{
					Errors: []transport.Diagnostic{
						{
							Code: transport.ManifestUnknownErrorCode,
						},
					},
				},
			},
			expected: expected{
				image: "",
				err:   oktetoErrors.ErrNotFound,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := client{
				config: tt.input.config,
				get: func(_ name.Reference, _ ...remote.Option) (*remote.Descriptor, error) {
					return tt.getConfig.descriptor, tt.getConfig.err
				},
			}
			image, err := c.GetDigest(tt.input.image)
			assert.Equal(t, tt.expected.image, image)
			assert.ErrorIs(t, err, tt.expected.err)
		})
	}
}

func TestGetOptions(t *testing.T) {
	type input struct {
		config fakeClientConfig
		image  string
	}
	var tests = []struct {
		name     string
		input    input
		expected []remote.Option
	}{
		{
			name: "our registry with own cert",
			input: input{
				config: fakeClientConfig{
					registryURL: "my-registry.com",
					userID:      "test",
					token:       "token",
					cert:        &x509.Certificate{},
				},
				image: "my-registry.com/test/test:latest",
			},
		},
		{
			name: "another registry with own cert",
			input: input{
				config: fakeClientConfig{
					registryURL: "my-registry.com",
					userID:      "test",
					token:       "token",
					cert:        &x509.Certificate{},
				},
				image: "another-registry.com/test/test:latest",
			},
		},
		{
			name: "our registry with insecure cert",
			input: input{
				config: fakeClientConfig{
					registryURL: "my-registry.com",
					userID:      "test",
					token:       "token",
					isInsecure:  true,
					cert:        &x509.Certificate{},
				},
				image: "my-registry.com/test/test:latest",
			},
		},
		{
			name: "another registry with insecure cert",
			input: input{
				config: fakeClientConfig{
					registryURL: "my-registry.com",
					userID:      "test",
					isInsecure:  true,
					token:       "token",
					cert:        &x509.Certificate{},
				},
				image: "another-registry.com/test/test:latest",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := client{
				config: tt.input.config,
			}
			ref, err := name.ParseReference(tt.input.image)
			assert.NoError(t, err)
			options := c.getOptions(ref)
			assert.Len(t, options, 2)
		})
	}
}

func TestGetTransport(t *testing.T) {
	type expected struct {
		input  fakeTLSDialArgs
		output fakeTLSDialArgs
	}
	var tests = []struct {
		name     string
		config   ClientConfigInterface
		expected []expected
	}{
		{
			name: "default",
			config: fakeClientConfig{
				registryURL: "registry.instance.foo",
				contextName: "https://okteto.instance.foo",
			},
			expected: []expected{
				{
					input:  fakeTLSDialArgs{network: "tcp", addr: "registry.instance.foo:443"},
					output: fakeTLSDialArgs{network: "tcp", addr: "registry.instance.foo:443", config: &tls.Config{}}, // skipcq: GO-S1020, GSC-G402
				},
				{
					input:  fakeTLSDialArgs{network: "tcp", addr: "okteto.instance.foo:443"},
					output: fakeTLSDialArgs{network: "tcp", addr: "okteto.instance.foo:443", config: &tls.Config{}}, // skipcq: GO-S1020, GSC-G402
				},
				{
					input:  fakeTLSDialArgs{network: "tcp", addr: "google.com:443"},
					output: fakeTLSDialArgs{network: "tcp", addr: "google.com:443", config: &tls.Config{}}, // skipcq: GO-S1020, GSC-G402
				},
			},
		},
		{
			name: "default with server name",
			config: fakeClientConfig{
				registryURL: "registry.instance.foo",
				contextName: "https://okteto.instance.foo",
				serverName:  "1.2.3.4:443",
			},
			expected: []expected{
				{
					input:  fakeTLSDialArgs{network: "tcp", addr: "registry.instance.foo:443"},
					output: fakeTLSDialArgs{network: "tcp", addr: "1.2.3.4:443", config: &tls.Config{ServerName: "registry.instance.foo"}}, // skipcq: GO-S1020, GSC-G402
				},
				{
					input:  fakeTLSDialArgs{network: "tcp", addr: "okteto.instance.foo:443"},
					output: fakeTLSDialArgs{network: "tcp", addr: "1.2.3.4:443", config: &tls.Config{ServerName: "okteto.instance.foo"}}, // skipcq: GO-S1020, GSC-G402
				},
				{
					input:  fakeTLSDialArgs{network: "tcp", addr: "google.com:443"},
					output: fakeTLSDialArgs{network: "tcp", addr: "google.com:443", config: &tls.Config{}}, // skipcq: GO-S1020, GSC-G402
				},
			},
		},
		{
			name: "public override",
			config: fakeClientConfig{
				registryURL: "registry.instance.foo",
				contextName: "https://instance.foo",
			},
			expected: []expected{
				{
					input:  fakeTLSDialArgs{network: "tcp", addr: "registry.instance.foo:443"},
					output: fakeTLSDialArgs{network: "tcp", addr: "registry.instance.foo:443", config: &tls.Config{}}, // skipcq: GO-S1020, GSC-G402
				},
				{
					input:  fakeTLSDialArgs{network: "tcp", addr: "instance.foo:443"},
					output: fakeTLSDialArgs{network: "tcp", addr: "instance.foo:443", config: &tls.Config{}}, // skipcq: GO-S1020, GSC-G402
				},
				{
					input:  fakeTLSDialArgs{network: "tcp", addr: "google.com:443"},
					output: fakeTLSDialArgs{network: "tcp", addr: "google.com:443", config: &tls.Config{}}, // skipcq: GO-S1020, GSC-G402
				},
			},
		},
		{
			name: "public override with server name",
			config: fakeClientConfig{
				registryURL: "registry.instance.foo",
				contextName: "https://instance.foo",
				serverName:  "1.2.3.4:443",
			},
			expected: []expected{
				{
					input:  fakeTLSDialArgs{network: "tcp", addr: "registry.instance.foo:443"},
					output: fakeTLSDialArgs{network: "tcp", addr: "1.2.3.4:443", config: &tls.Config{ServerName: "registry.instance.foo"}}, // skipcq: GO-S1020, GSC-G402
				},
				{
					input:  fakeTLSDialArgs{network: "tcp", addr: "instance.foo:443"},
					output: fakeTLSDialArgs{network: "tcp", addr: "1.2.3.4:443", config: &tls.Config{ServerName: "instance.foo"}}, // skipcq: GO-S1020, GSC-G402
				},
				{
					input:  fakeTLSDialArgs{network: "tcp", addr: "google.com:443"},
					output: fakeTLSDialArgs{network: "tcp", addr: "google.com:443", config: &tls.Config{}}, // skipcq: GO-S1020, GSC-G402
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, e := range tt.expected {
				dialer := &fakeTLSDial{}
				client := client{config: tt.config, tlsDial: dialer.tlsDial}

				roundTripper := client.getTransport()
				transport, ok := roundTripper.(*http.Transport)
				assert.True(t, ok, "getTransport() is not *http.Transport")

				conn, err := transport.DialTLSContext(context.TODO(), e.input.network, e.input.addr)
				assert.NoError(t, err, "transport.DialTLSContext returned error")
				assert.NotNil(t, conn, "transport.DialTLSContext returned nil net.Conn")

				assert.Len(t, dialer.requests, 1)

				assert.Equal(t, e.output.network, dialer.requests[0].network)
				assert.Equal(t, e.output.addr, dialer.requests[0].addr)

				assert.NotNil(t, dialer.requests[0].config)
				assert.Equal(t, e.output.config.ServerName, dialer.requests[0].config.ServerName)
			}
		})
	}
}

func Test_GetDescriptor(t *testing.T) {
	type input struct {
		config fakeClientConfig
		image  string
	}
	type getConfig struct {
		descriptor *remote.Descriptor
		err        error
	}
	type expected struct {
		descriptor *remote.Descriptor
		err        error
	}
	var tests = []struct {
		name      string
		input     input
		expected  expected
		getConfig getConfig
	}{
		{
			name: "failed to parse reference",
			input: input{
				image: "broken image uri",
			},
			expected: expected{
				descriptor: nil,
				err:        fmt.Errorf("could not parse reference: broken image uri"),
			},
			getConfig: getConfig{
				descriptor: nil,
				err:        nil,
			},
		},
		{
			name: "not found",
			input: input{
				image: "okteto/test:latest",
			},
			expected: expected{
				descriptor: nil,
				err:        fmt.Errorf("error getting image descriptor: %w", oktetoErrors.ErrNotFound),
			},
			getConfig: getConfig{
				descriptor: nil,
				err:        oktetoErrors.ErrNotFound,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := client{
				config: tt.input.config,
				get: func(_ name.Reference, _ ...remote.Option) (*remote.Descriptor, error) {
					return tt.getConfig.descriptor, tt.getConfig.err
				},
			}
			descriptor, err := c.GetDescriptor(tt.input.image)
			assert.Equal(t, tt.expected.descriptor, descriptor)
			if tt.expected.err != nil {
				assert.EqualError(t, err, tt.expected.err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_Write(t *testing.T) {
	type input struct {
		ref   name.Reference
		image containerv1.Image
	}
	type writeConfig struct {
		err error
	}
	type expected struct {
		err error
	}
	var tests = []struct {
		name        string
		input       input
		expected    expected
		client      fakeClient
		writeConfig writeConfig
	}{
		{
			name: "failed to write",
			input: input{
				ref:   name.Reference(nil),
				image: containerv1.Image(nil),
			},
			expected: expected{
				err: assert.AnError,
			},
			writeConfig: writeConfig{
				err: assert.AnError,
			},
		},
		{
			name: "success",
			input: input{
				ref:   name.Reference(nil),
				image: containerv1.Image(nil),
			},
			expected: expected{
				err: nil,
			},
			writeConfig: writeConfig{
				err: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := client{
				write: func(_ name.Reference, _ containerv1.Image, _ ...remote.Option) error {
					return tt.writeConfig.err
				},
			}
			err := c.write(tt.input.ref, tt.input.image)
			if tt.expected.err != nil {
				assert.EqualError(t, err, tt.expected.err.Error())
			} else {
				assert.NoError(t, err)
			}

		})
	}
}
