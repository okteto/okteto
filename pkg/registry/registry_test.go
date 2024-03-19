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
	"fmt"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
)

type FakeConfig struct {
	err                         error
	ContextCertificate          *x509.Certificate
	externalRegistryCredentials [2]string
	GlobalNamespace             string
	Namespace                   string
	RegistryURL                 string
	UserID                      string
	Token                       string
	ServerName                  string
	ContextName                 string
	InsecureSkipTLSVerifyPolicy bool
	IsOktetoClusterCfg          bool
}

func (fc FakeConfig) IsOktetoCluster() bool               { return fc.IsOktetoClusterCfg }
func (fc FakeConfig) GetGlobalNamespace() string          { return fc.GlobalNamespace }
func (fc FakeConfig) GetNamespace() string                { return fc.Namespace }
func (fc FakeConfig) GetRegistryURL() string              { return fc.RegistryURL }
func (fc FakeConfig) GetUserID() string                   { return fc.UserID }
func (fc FakeConfig) GetToken() string                    { return fc.Token }
func (fc FakeConfig) IsInsecureSkipTLSVerifyPolicy() bool { return fc.InsecureSkipTLSVerifyPolicy }
func (fc FakeConfig) GetContextCertificate() (*x509.Certificate, error) {
	return fc.ContextCertificate, fc.err
}
func (fc FakeConfig) GetServerNameOverride() string { return fc.ServerName }
func (fc FakeConfig) GetContextName() string        { return fc.ContextName }
func (fc FakeConfig) GetExternalRegistryCredentials(_ string) (string, string, error) {
	return fc.externalRegistryCredentials[0], fc.externalRegistryCredentials[1], fc.err
}

func TestGetImageTagWithDigest(t *testing.T) {
	type expected struct {
		err      error
		imageTag string
	}
	type clientConfig struct {
		err    error
		digest string
	}
	type config struct {
		config       configInterface
		clientConfig clientConfig
		input        string
	}
	var tests = []struct {
		input    config
		expected expected
		name     string
	}{
		{
			name: "get no error",
			input: config{
				input: "okteto/test",
				config: FakeConfig{
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
				config: FakeConfig{
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
				imageCtrl: NewImageCtrl(tt.input.config),
				client: fakeClient{
					GetImageDigest: getDigest{
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

func TestHasPushAccess(t *testing.T) {
	type expected struct {
		err       error
		hasAccess bool
	}
	type clientConfig struct {
		err       error
		hasAccess bool
	}
	type config struct {
		clientConfig   clientConfig
		registryConfig FakeConfig
	}
	var tests = []struct {
		expected expected
		name     string
		config   config
	}{
		{
			name: "not in okteto",
			config: config{
				registryConfig: FakeConfig{
					IsOktetoClusterCfg: false,
				},
				clientConfig: clientConfig{
					hasAccess: false,
					err:       nil,
				},
			},
			expected: expected{
				hasAccess: false,
				err:       nil,
			},
		},
		{
			name: "in okteto - with access",
			config: config{
				registryConfig: FakeConfig{
					IsOktetoClusterCfg: true,
				},
				clientConfig: clientConfig{
					hasAccess: true,
					err:       nil,
				},
			},
			expected: expected{
				hasAccess: true,
				err:       nil,
			},
		},
		{
			name: "in okteto - no access",
			config: config{
				registryConfig: FakeConfig{
					IsOktetoClusterCfg: true,
				},
				clientConfig: clientConfig{
					hasAccess: false,
					err:       assert.AnError,
				},
			},
			expected: expected{
				hasAccess: false,
				err:       assert.AnError,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			or := OktetoRegistry{
				imageCtrl: NewImageCtrl(tt.config.registryConfig),
				config:    tt.config.registryConfig,
				client: fakeClient{
					HasPushAcces: hasPushAccess{
						Result: tt.config.clientConfig.hasAccess,
						Err:    tt.config.clientConfig.err,
					},
				},
			}

			result, err := or.HasGlobalPushAccess()
			assert.Equal(t, tt.expected.hasAccess, result)
			assert.ErrorIs(t, err, tt.expected.err)
		})
	}
}

func TestGetImageMetadata(t *testing.T) {
	type expected struct {
		err      error
		metadata ImageMetadata
	}
	type clientConfig struct {
		getConfig getConfig
		getDigest getDigest
	}
	type config struct {
		clientConfig clientConfig
		config       configInterface
		input        string
	}
	var tests = []struct {
		input    config
		name     string
		expected expected
	}{
		{
			name: "getDigest/getImageMetadata no error",
			input: config{
				input: "okteto/test",
				config: FakeConfig{
					IsOktetoClusterCfg: false,
					ContextCertificate: &x509.Certificate{},
				},
				clientConfig: clientConfig{
					getDigest: getDigest{
						Result: "thisisatest",
						Err:    nil,
					},
					getConfig: getConfig{
						Result: &v1.ConfigFile{
							Config: v1.Config{
								ExposedPorts: map[string]struct{}{
									"8080/tcp": {},
								},
								Cmd:        []string{"sh", "-c", "python start"},
								WorkingDir: "/usr/src/app",
							},
						},
						Err: nil,
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
				config: FakeConfig{
					IsOktetoClusterCfg: false,
					ContextCertificate: &x509.Certificate{},
				},
				clientConfig: clientConfig{
					getDigest: getDigest{
						Result: "",
						Err:    assert.AnError,
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
				config: FakeConfig{
					IsOktetoClusterCfg: false,
					ContextCertificate: &x509.Certificate{},
				},
				clientConfig: clientConfig{
					getDigest: getDigest{
						Result: "thisisatest",
						Err:    nil,
					},
					getConfig: getConfig{
						Result: nil,
						Err:    assert.AnError,
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
				imageCtrl: NewImageCtrl(tt.input.config),
				client: fakeClient{
					GetImageDigest: tt.input.clientConfig.getDigest,
					GetConfig:      tt.input.clientConfig.getConfig,
				},
			}

			result, err := or.GetImageMetadata(tt.input.input)
			assert.Equal(t, tt.expected.metadata, result)
			assert.ErrorIs(t, err, tt.expected.err)
		})
	}
}

func Test_OktetoRegistry_CloneGlobalImageToDev(t *testing.T) {
	type expected struct {
		err   error
		image string
	}
	type config struct {
		config configInterface
		image  string
	}
	var tests = []struct {
		client   fakeClient
		input    config
		expected expected
		name     string
	}{
		{
			name: "not a global image repository returns error",
			input: config{
				image: "this.is.my.okteto.registry/test-ns/test-repo@sha-256:123456789",
				config: FakeConfig{
					RegistryURL:        "this.is.my.okteto.registry",
					GlobalNamespace:    "test-global",
					IsOktetoClusterCfg: true,
					Namespace:          "test-ns",
				},
			},
			expected: expected{
				image: "",
				err:   fmt.Errorf("image repository 'test-ns/test-repo' is not in the global registry"),
			},
		},
		{
			name: "fail to get global image descriptor",
			input: config{
				image: "this.is.my.okteto.registry/test-global/test-repo@sha256:123",
				config: FakeConfig{
					RegistryURL:        "this.is.my.okteto.registry",
					GlobalNamespace:    "test-global",
					IsOktetoClusterCfg: true,
					Namespace:          "test-ns",
				},
			},
			client: fakeClient{
				MockGetDescriptor: mockGetDescriptor{
					Result: nil,
					Err:    assert.AnError,
				},
			},
			expected: expected{
				image: "",
				err:   assert.AnError,
			},
		},
		{
			name: "success",
			input: config{
				image: "this.is.my.okteto.registry/test-global/test-repo@sha256:123",
				config: FakeConfig{
					RegistryURL:        "this.is.my.okteto.registry",
					GlobalNamespace:    "test-global",
					IsOktetoClusterCfg: true,
					Namespace:          "test-ns",
				},
			},
			client: fakeClient{
				GetImageDigest: getDigest{
					Result: "sha256:123456789",
				},
				MockGetDescriptor: mockGetDescriptor{
					Result: &remote.Descriptor{
						Descriptor: v1.Descriptor{},
						Manifest:   nil,
					},
				},
				MockWrite: mockWrite{
					Err: nil,
				},
			},
			expected: expected{
				image: "this.is.my.okteto.registry/test-ns/test-repo@sha256:123456789",
				err:   nil,
			},
		},
		{
			name: "withErrorGettingDevImageDigest",
			input: config{
				image: "this.is.my.okteto.registry/test-global/test-repo@sha256:123",
				config: FakeConfig{
					RegistryURL:        "this.is.my.okteto.registry",
					GlobalNamespace:    "test-global",
					IsOktetoClusterCfg: true,
					Namespace:          "test-ns",
				},
			},
			client: fakeClient{
				GetImageDigest: getDigest{
					Err: assert.AnError,
				},
				MockGetDescriptor: mockGetDescriptor{
					Result: &remote.Descriptor{
						Descriptor: v1.Descriptor{},
						Manifest:   nil,
					},
				},
				MockWrite: mockWrite{
					Err: nil,
				},
			},
			expected: expected{
				image: "this.is.my.okteto.registry/test-ns/test-repo:okteto",
				err:   nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			or := OktetoRegistry{
				imageCtrl: NewImageCtrl(tt.input.config),
				config:    tt.input.config,
				client:    tt.client,
			}

			result, err := or.CloneGlobalImageToDev(tt.input.image)
			assert.Equal(t, tt.expected.image, result)
			if tt.expected.err != nil && err != nil {
				assert.Equal(t, err.Error(), tt.expected.err.Error())
			}
		})
	}
}

func Test_IsOktetoRegistry(t *testing.T) {
	type input struct {
		config configInterface
		image  string
	}
	var tests = []struct {
		input input
		name  string
		want  bool
	}{
		{
			name: "is-dev-registry",
			input: input{
				image: "okteto.dev/image",
				config: FakeConfig{
					RegistryURL:        "this.is.my.okteto.registry",
					IsOktetoClusterCfg: true,
				},
			},
			want: true,
		},
		{
			name: "is-global-registry",
			input: input{
				image: "okteto.global/image",
				config: FakeConfig{
					RegistryURL:        "this.is.my.okteto.registry",
					IsOktetoClusterCfg: true,
				},
			},
			want: true,
		},
		{
			name: "is-expanded-dev-registry",
			input: input{
				image: "this.is.my.okteto.registry/user/image",
				config: FakeConfig{
					RegistryURL:        "this.is.my.okteto.registry",
					IsOktetoClusterCfg: true,
				},
			},
			want: true,
		},
		{
			name: "is-not-dev-registry",
			input: input{
				image: "other-image/image",
				config: FakeConfig{
					RegistryURL:        "this.is.my.okteto.registry",
					IsOktetoClusterCfg: true,
				},
			},
			want: false,
		},
		{
			name: "is-dev-registry but cluster is not managed by okteto",
			input: input{
				image: "okteto.dev/image",
				config: FakeConfig{
					RegistryURL:        "this.is.my.okteto.registry",
					IsOktetoClusterCfg: false,
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			or := NewOktetoRegistry(tt.input.config)

			result := or.IsOktetoRegistry(tt.input.image)
			assert.Equal(t, tt.want, result)
		})
	}
}

func Test_IsGlobal(t *testing.T) {
	type input struct {
		config configInterface
		image  string
	}
	var tests = []struct {
		input input
		name  string
		want  bool
	}{
		{
			name: "is-dev-registry",
			input: input{
				image: "okteto.dev/image",
				config: FakeConfig{
					RegistryURL:        "this.is.my.okteto.registry",
					GlobalNamespace:    "test",
					IsOktetoClusterCfg: true,
				},
			},
			want: false,
		},
		{
			name: "is-global-registry",
			input: input{
				image: "okteto.global/image",
				config: FakeConfig{
					RegistryURL:        "this.is.my.okteto.registry",
					GlobalNamespace:    "test",
					IsOktetoClusterCfg: true,
				},
			},
			want: true,
		},
		{
			name: "is-expanded-global-registry",
			input: input{
				image: "this.is.my.okteto.registry/test/image",
				config: FakeConfig{
					RegistryURL:        "this.is.my.okteto.registry",
					GlobalNamespace:    "test",
					IsOktetoClusterCfg: true,
				},
			},
			want: true,
		},
		{
			name: "is-not-dev-registry",
			input: input{
				image: "other-image/image",
				config: FakeConfig{
					RegistryURL:        "this.is.my.okteto.registry",
					GlobalNamespace:    "test",
					IsOktetoClusterCfg: true,
				},
			},
			want: false,
		},
		{
			name: "avoid false positives when global namespace is same as dev namespace prefix",
			input: input{
				image: "this.is.my.okteto.registry/test-user000/image",
				config: FakeConfig{
					RegistryURL:        "this.is.my.okteto.registry",
					GlobalNamespace:    "test",
					IsOktetoClusterCfg: true,
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			or := NewOktetoRegistry(tt.input.config)

			result := or.IsGlobalRegistry(tt.input.image)
			assert.Equal(t, tt.want, result)
		})
	}
}

func Test_GetImageTag(t *testing.T) {
	type input struct {
		config    configInterface
		image     string
		service   string
		namespace string
	}
	var tests = []struct {
		name     string
		input    input
		expected string
	}{
		{
			name: "not-in-okteto",
			input: input{
				image:     "okteto/hello",
				service:   "service",
				namespace: "namespace",
				config: FakeConfig{
					RegistryURL: "",
				},
			},
			expected: "okteto/hello:okteto",
		},
		{
			name: "in-okteto-image-in-okteto",
			input: input{
				image:     "my-registry.com/hello",
				service:   "service",
				namespace: "namespace",
				config: FakeConfig{
					RegistryURL:        "my-registry.com",
					IsOktetoClusterCfg: true,
				},
			},
			expected: "my-registry.com/hello",
		},
		{
			name: "in-okteto-image-in-okteto",
			input: input{
				image:     "hello",
				service:   "service",
				namespace: "namespace",
				config: FakeConfig{
					RegistryURL:        "my-registry.com",
					IsOktetoClusterCfg: true,
				},
			},
			expected: "my-registry.com/namespace/service:okteto",
		},
		{
			name: "in-okteto-image-not-in-okteto",
			input: input{
				image:     "okteto/hello",
				service:   "service",
				namespace: "namespace",
				config: FakeConfig{
					RegistryURL: "my-registry.com",
				},
			},
			expected: "my-registry.com/namespace/service:okteto",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			or := NewOktetoRegistry(tt.input.config)
			result := or.GetImageTag(tt.input.image, tt.input.service, tt.input.namespace)

			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetImageReference(t *testing.T) {
	var tests = []struct {
		name     string
		input    string
		expected OktetoImageReference
	}{
		{
			name:  "with registry",
			input: "my-registry.com/okteto/hello:okteto",
			expected: OktetoImageReference{
				Registry: "my-registry.com",
				Repo:     "okteto/hello",
				Tag:      "okteto",
				Image:    "my-registry.com/okteto/hello:okteto",
			},
		},
		{
			name:  "without registry",
			input: "okteto/hello:okteto",
			expected: OktetoImageReference{
				Registry: "index.docker.io",
				Repo:     "okteto/hello",
				Tag:      "okteto",
				Image:    "okteto/hello:okteto",
			},
		},
		{
			name:  "without tag",
			input: "my-registry.com/okteto/hello",
			expected: OktetoImageReference{
				Registry: "my-registry.com",
				Repo:     "okteto/hello",
				Tag:      "latest",
				Image:    "my-registry.com/okteto/hello",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			or := OktetoRegistry{}
			result, err := or.GetImageReference(tt.input)

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
