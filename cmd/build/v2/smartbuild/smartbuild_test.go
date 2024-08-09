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
	"github.com/okteto/okteto/pkg/vars"
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

func (frc fakeRegistryController) GetDevImageFromGlobal(image string) string { return image }

func (frc fakeRegistryController) IsGlobalRegistry(string) bool { return frc.isGlobalRegistry }
func (frc fakeRegistryController) IsOktetoRegistry(string) bool { return false }
func (fr fakeRegistryController) Clone(from, to string) (string, error) {
	return from, nil
}

type fakeHasher struct {
	err  error
	hash string
}

func (fh fakeHasher) hashProjectCommit(*build.Info) (string, error) { return fh.hash, fh.err }
func (fh fakeHasher) hashWithBuildContext(*build.Info, string) string {
	return fh.hash
}

type fakeVarManager struct{}

func (*fakeVarManager) MaskVar(string) {}

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

func Test_getBuildHashFromCommit(t *testing.T) {
	vars.GlobalVarManager = vars.NewVarsManager(&fakeVarManager{})
	vars.GlobalVarManager.AddFlagVar("BAR", "bar")

	fs := afero.NewMemMapFs()
	err := afero.WriteFile(fs, "secret", []byte("bar"), 0600)
	assert.NoError(t, err)

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
		// TODO: discuss with the team about this unit test
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

func TestCloneGlobalImageToDev(t *testing.T) {
	type input struct {
		from string
		to   string
	}
	type output struct {
		err      error
		devImage string
	}

	tests := []struct {
		input  input
		name   string
		output output
	}{
		{
			name: "Global Registry",
			input: input{
				from: "okteto.global/myimage",
				to:   "",
			},
			output: output{
				devImage: "okteto.global/myimage",
			},
		},
		{
			name: "Non-Global Registry",
			input: input{
				from: "okteto.dev/myimage",
				to:   "",
			},
			output: output{
				devImage: "okteto.dev/myimage",
				err:      nil,
			},
		},
		{
			name: "Global Registry with image set in buildInfo",
			input: input{
				from: "okteto.global/myimage:sha",
				to:   "okteto.dev/myimage:1.0",
			},
			output: output{
				devImage: "okteto.global/myimage:sha",
			},
		},
		{
			name: "Non-Global Registry with image set in buildInfo",
			input: input{
				from: "okteto.dev/myimage:sha",
				to:   "okteto.dev/myimage:1.0",
			},
			output: output{
				devImage: "okteto.dev/myimage:sha",
				err:      nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := Ctrl{
				registryController: fakeRegistryController{
					isGlobalRegistry: tt.input.from == "okteto.global/myimage",
					err:              tt.output.err,
				},
				ioCtrl: io.NewIOController(),
			}

			devImage, err := ctrl.CloneGlobalImageToDev(tt.input.from, tt.input.to)

			assert.Equal(t, tt.output.devImage, devImage)
			assert.Equal(t, tt.output.err, err)
		})
	}
}
