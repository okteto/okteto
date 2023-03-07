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
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/registry/registry/fake"
	"github.com/stretchr/testify/assert"
)

func TestGetDigest(t *testing.T) {
	unautorizedErr := &transport.Error{
		Errors: []transport.Diagnostic{
			{
				Code: transport.UnauthorizedErrorCode,
			},
		},
	}

	type input struct {
		config fake.FakeConfig
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
				config: fake.FakeConfig{
					ContextCertificate: &x509.Certificate{},
				},
				image: "okteto/test:latest",
			},
			getConfig: getConfig{
				descriptor: &remote.Descriptor{
					Descriptor: v1.Descriptor{
						Digest: v1.Hash{
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
				config: fake.FakeConfig{
					ContextCertificate: &x509.Certificate{},
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
				config: fake.FakeConfig{
					ContextCertificate: &x509.Certificate{},
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
		config fake.FakeConfig
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
				config: fake.FakeConfig{
					RegistryURL:        "my-registry.com",
					UserID:             "test",
					Token:              "token",
					ContextCertificate: &x509.Certificate{},
				},
				image: "my-registry.com/test/test:latest",
			},
		},
		{
			name: "another registry with own cert",
			input: input{
				config: fake.FakeConfig{
					RegistryURL:        "my-registry.com",
					UserID:             "test",
					Token:              "token",
					ContextCertificate: &x509.Certificate{},
				},
				image: "another-registry.com/test/test:latest",
			},
		},
		{
			name: "our registry with insecure cert",
			input: input{
				config: fake.FakeConfig{
					RegistryURL:                 "my-registry.com",
					UserID:                      "test",
					Token:                       "token",
					InsecureSkipTLSVerifyPolicy: true,
					ContextCertificate:          &x509.Certificate{},
				},
				image: "my-registry.com/test/test:latest",
			},
		},
		{
			name: "another registry with insecure cert",
			input: input{
				config: fake.FakeConfig{
					RegistryURL:                 "my-registry.com",
					UserID:                      "test",
					InsecureSkipTLSVerifyPolicy: true,
					Token:                       "token",
					ContextCertificate:          &x509.Certificate{},
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
