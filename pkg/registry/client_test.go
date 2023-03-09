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
	"crypto/x509"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	containerv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/stretchr/testify/assert"
)

// FakeClient has everything needed to set up a test faking API calls
type fakeClient struct {
	GetImageDigest getDigest
	GetConfig      getConfig
	HasPushAcces   hasPushAccess
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

func (fc fakeClient) HasPushAccess(image string) (bool, error) {
	return fc.HasPushAcces.Result, fc.HasPushAcces.Err
}

type fakeClientConfig struct {
	registryURL string
	userID      string
	token       string
	isInsecure  bool
	cert        *x509.Certificate
}

func (f fakeClientConfig) GetRegistryURL() string                            { return f.registryURL }
func (f fakeClientConfig) GetUserID() string                                 { return f.userID }
func (f fakeClientConfig) GetToken() string                                  { return f.token }
func (f fakeClientConfig) IsInsecureSkipTLSVerifyPolicy() bool               { return f.isInsecure }
func (f fakeClientConfig) GetContextCertificate() (*x509.Certificate, error) { return f.cert, nil }

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
