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

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/okteto/okteto/pkg/registry/registry/fake"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
)

func TestGetImageTagWithDigest(t *testing.T) {
	type expected struct {
		imageTag string
		err      error
	}
	type clientConfig struct {
		digest string
		err    error
	}
	type config struct {
		input        string
		config       configInterface
		clientConfig clientConfig
	}
	var tests = []struct {
		name     string
		input    config
		expected expected
	}{
		{
			name: "get no error",
			input: config{
				input: "okteto/test",
				config: fake.FakeConfig{
					IsOktetoClusterCfg: false,
					ContextCertificate: &x509.Certificate{},
				},
				clientConfig: clientConfig{
					digest: "thisisatest",
					err:    nil,
				},
			},
			expected: expected{
				imageTag: "docker.io/okteto/test@thisisatest",
				err:      nil,
			},
		},
		{
			name: "get with error",
			input: config{
				input: "okteto/test",
				config: fake.FakeConfig{
					IsOktetoClusterCfg: false,
					ContextCertificate: &x509.Certificate{},
				},
				clientConfig: clientConfig{
					digest: "thisisatest",
					err:    assert.AnError,
				},
			},
			expected: expected{
				imageTag: "",
				err:      assert.AnError,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			or := OktetoRegistry{
				imageCtrl: newImageCtrl(tt.input.config),
				client: fake.FakeClient{
					GetImageDigest: fake.GetDigest{
						Result: tt.input.clientConfig.digest,
						Err:    tt.input.clientConfig.err,
					},
				},
			}

			result, err := or.GetImageTagWithDigest(tt.input.input)
			assert.Equal(t, tt.expected.imageTag, result)
			assert.ErrorIs(t, err, tt.expected.err)
		})
	}
}

func TestGetImageMetadata(t *testing.T) {
	type expected struct {
		metadata ImageMetadata
		err      error
	}
	type getDigest struct {
		digest string
		err    error
	}
	type getConfig struct {
		cfg *v1.ConfigFile
		err error
	}
	type clientConfig struct {
		getDigest getDigest
		getConfig getConfig
	}
	type config struct {
		input        string
		config       configInterface
		clientConfig clientConfig
	}
	var tests = []struct {
		name     string
		input    config
		expected expected
	}{
		{
			name: "getDigest/getImageMetadata no error",
			input: config{
				input: "okteto/test",
				config: fake.FakeConfig{
					IsOktetoClusterCfg: false,
					ContextCertificate: &x509.Certificate{},
				},
				clientConfig: clientConfig{
					getDigest: getDigest{
						digest: "thisisatest",
						err:    nil,
					},
					getConfig: getConfig{
						cfg: &v1.ConfigFile{
							Config: v1.Config{
								ExposedPorts: map[string]struct{}{
									"8080/tcp": {},
								},
								Cmd:        []string{"sh", "-c", "python start"},
								WorkingDir: "/usr/src/app",
							},
						},
						err: nil,
					},
				},
			},
			expected: expected{
				metadata: ImageMetadata{
					Image:   "docker.io/okteto/test@thisisatest",
					CMD:     []string{"sh", "-c", "python start"},
					Workdir: "/usr/src/app",
					Ports:   []Port{{ContainerPort: 8080, Protocol: apiv1.ProtocolTCP}},
				},
				err: nil,
			},
		},
		{
			name: "getDigest with error",
			input: config{
				input: "okteto/test",
				config: fake.FakeConfig{
					IsOktetoClusterCfg: false,
					ContextCertificate: &x509.Certificate{},
				},
				clientConfig: clientConfig{
					getDigest: getDigest{
						digest: "",
						err:    assert.AnError,
					},
				},
			},
			expected: expected{
				metadata: ImageMetadata{},
				err:      assert.AnError,
			},
		},
		{
			name: "getDigest/getImageMetadata no error",
			input: config{
				input: "okteto/test",
				config: fake.FakeConfig{
					IsOktetoClusterCfg: false,
					ContextCertificate: &x509.Certificate{},
				},
				clientConfig: clientConfig{
					getDigest: getDigest{
						digest: "thisisatest",
						err:    nil,
					},
					getConfig: getConfig{
						cfg: nil,
						err: assert.AnError,
					},
				},
			},
			expected: expected{
				metadata: ImageMetadata{},
				err:      assert.AnError,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			or := OktetoRegistry{
				imageCtrl: newImageCtrl(tt.input.config),
				client: fake.FakeClient{
					GetImageDigest: fake.GetDigest{
						Result: tt.input.clientConfig.getDigest.digest,
						Err:    tt.input.clientConfig.getDigest.err,
					},
					GetConfig: fake.GetConfig{
						Result: tt.input.clientConfig.getConfig.cfg,
						Err:    tt.input.clientConfig.getConfig.err,
					},
				},
			}

			result, err := or.GetImageMetadata(tt.input.input)
			assert.Equal(t, tt.expected.metadata, result)
			assert.ErrorIs(t, err, tt.expected.err)
		})
	}
}
