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

package smartbuild

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

type fakeConfigRepo struct {
	err  error
	sha  string
	diff string
}

func (fcr fakeConfigRepo) GetSHA() (string, error)                { return fcr.sha, fcr.err }
func (fcr fakeConfigRepo) GetLatestDirSHA(string) (string, error) { return fcr.sha, fcr.err }
func (fcr fakeConfigRepo) GetDiffHash(string) (string, error)     { return fcr.diff, fcr.err }

type fakeRegistryController struct {
	err              error
	isGlobalRegistry bool
}

func (frc fakeRegistryController) CloneGlobalImageToDev(image string) (string, error) {
	return image, frc.err
}
func (frc fakeRegistryController) IsGlobalRegistry(string) bool { return frc.isGlobalRegistry }

type fakeHasher struct {
	err  error
	hash string
}

func (fh fakeHasher) hashProjectCommit(*build.Info) (string, error) { return fh.hash, fh.err }
func (fh fakeHasher) hashWithBuildContext(*build.Info, string) string {
	return fh.hash
}
func (fh fakeHasher) getServiceShaInCache(string) string { return fh.hash }

func TestNewSmartBuildCtrl(t *testing.T) {
	type input struct {
		isEnabledValue string
	}
	type output struct {
		isEnabled bool
	}

	tests := []struct {
		name   string
		input  input
		output output
	}{
		{
			name: "Default Configuration",
			input: input{
				isEnabledValue: "",
			},
			output: output{
				isEnabled: true,
			},
		},
		{
			name: "Environment Variable Disabled",
			input: input{
				isEnabledValue: "false",
			},
			output: output{
				isEnabled: false,
			},
		},
		{
			name: "Environment variable Enabled",
			input: input{
				isEnabledValue: "true",
			},
			output: output{
				isEnabled: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(OktetoEnableSmartBuildEnvVar, tt.input.isEnabledValue)

			ctrl := NewSmartBuildCtrl(&fakeConfigRepo{}, &fakeRegistryController{}, afero.NewMemMapFs(), io.NewIOController())

			assert.Equal(t, tt.output.isEnabled, ctrl.IsEnabled())
		})
	}
}

func TestGetProjectHash(t *testing.T) {
	type input struct {
		err  error
		hash string
	}
	type output struct {
		err  error
		hash string
	}

	tests := []struct {
		name   string
		input  input
		output output
	}{
		{
			name: "correct hash",
			input: input{
				hash: "hash",
				err:  nil,
			},
			output: output{
				hash: "hash",
				err:  nil,
			},
		},
		{
			name: "error",
			input: input{
				hash: "",
				err:  assert.AnError,
			},
			output: output{
				hash: "",
				err:  assert.AnError,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sbc := Ctrl{
				ioCtrl: io.NewIOController(),
				hasher: fakeHasher{
					hash: tt.input.hash,
					err:  tt.input.err,
				},
			}
			out, err := sbc.GetProjectHash(&build.Info{})
			assert.Equal(t, tt.output.hash, out)
			assert.ErrorIs(t, err, tt.output.err)
		})
	}
}

func TestGetServiceHash(t *testing.T) {
	service := "fake-service"
	sbc := Ctrl{
		ioCtrl: io.NewIOController(),
		hasher: fakeHasher{
			hash: "hash",
		},
	}
	out := sbc.GetServiceHash(&build.Info{}, service)
	assert.Equal(t, "hash", out)
}

func TestGetBuildHash(t *testing.T) {
	service := "fake-service"
	sbc := Ctrl{
		ioCtrl: io.NewIOController(),
		hasher: fakeHasher{
			hash: "hash",
		},
	}
	out := sbc.GetBuildHash(&build.Info{}, service)
	assert.Equal(t, "hash", out)
}

func TestCloneGlobalImageToDev(t *testing.T) {
	type input struct {
		err      error
		isGlobal bool
	}
	type output struct {
		err  error
		hash string
	}

	tests := []struct {
		name   string
		input  input
		output output
	}{
		{
			name: "isGlobal - err",
			input: input{
				isGlobal: true,
				err:      assert.AnError,
			},
			output: output{
				hash: "",
				err:  assert.AnError,
			},
		},
		{
			name: "isGlobal - no error",
			input: input{
				isGlobal: true,
				err:      nil,
			},
			output: output{
				hash: "test",
				err:  nil,
			},
		},
		{
			name: "not global",
			input: input{
				isGlobal: false,
			},
			output: output{
				hash: "test",
				err:  nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sbc := Ctrl{
				ioCtrl: io.NewIOController(),
				registryController: fakeRegistryController{
					err:              tt.input.err,
					isGlobalRegistry: tt.input.isGlobal,
				},
			}
			out, err := sbc.CloneGlobalImageToDev("test")
			assert.Equal(t, tt.output.hash, out)
			assert.ErrorIs(t, err, tt.output.err)
		})
	}
}

func Test_getBuildHashFromCommit(t *testing.T) {
	fs := afero.NewMemMapFs()
	err := afero.WriteFile(fs, "secret", []byte("bar"), 0600)
	assert.NoError(t, err)
	t.Setenv("BAR", "bar")
	type input struct {
		buildInfo *build.Info
		repo      fakeConfigRepo
	}
	tt := []struct {
		name        string
		expected    string
		expectedErr error
		input       input
	}{
		{
			name: "valid commit",
			input: input{
				repo: fakeConfigRepo{
					sha: "1234567890",
					err: nil,
				},
				buildInfo: &build.Info{
					Args: build.Args{
						{
							Name:  "foo",
							Value: "bar",
						},
						{
							Name:  "key",
							Value: "value",
						},
					},
					Target: "target",
					Secrets: build.Secrets{
						"secret": "secret",
					},
					Context:    "context",
					Dockerfile: "dockerfile",
					Image:      "image",
				},
			},
			expected: "commit:1234567890;target:target;build_args:foo=bar;key=value;secrets:secret=secret;context:context;dockerfile_content:;diff:;image:image;",
		},
		{
			name: "invalid commit",
			input: input{
				repo: fakeConfigRepo{
					sha: "",
					err: assert.AnError,
				},
				buildInfo: &build.Info{
					Args: build.Args{
						{
							Name:  "foo",
							Value: "bar",
						},
						{
							Name:  "key",
							Value: "value",
						},
					},
					Target: "target",
					Secrets: build.Secrets{
						"secret": "secret",
					},
					Context:    "context",
					Dockerfile: "dockerfile",
					Image:      "image",
				},
			},
			expected:    "",
			expectedErr: assert.AnError,
		},
		{
			name: "invalid commit and no args",
			input: input{
				repo: fakeConfigRepo{
					sha: "",
					err: assert.AnError,
				},
				buildInfo: &build.Info{
					Args:   build.Args{},
					Target: "target",
					Secrets: build.Secrets{
						"secret": "secret",
					},
					Context:    "context",
					Dockerfile: "dockerfile",
					Image:      "image",
				},
			},
			expected:    "",
			expectedErr: assert.AnError,
		},
		{
			name: "arg with expansion",
			input: input{
				repo: fakeConfigRepo{
					sha: "123",
				},
				buildInfo: &build.Info{
					Args: build.Args{
						{
							Name:  "foo",
							Value: "$BAR",
						},
					},
					Target: "target",
					Secrets: build.Secrets{
						"secret": "secret",
					},
					Context:    "context",
					Dockerfile: "dockerfile",
					Image:      "image",
				},
			},
			expected: "commit:123;target:target;build_args:foo=bar;secrets:secret=secret;context:context;dockerfile_content:;diff:;image:image;",
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got, err := newServiceHasher(fakeConfigRepo{
				sha: tc.input.repo.sha,
				err: tc.input.repo.err,
			}, afero.NewMemMapFs()).hashProjectCommit(tc.input.buildInfo)
			assert.ErrorIs(t, err, tc.expectedErr)
			if tc.expected != "" {
				expectedHash := sha256.Sum256([]byte(tc.expected))
				assert.Equal(t, hex.EncodeToString(expectedHash[:]), got)
			}
		})
	}
}
