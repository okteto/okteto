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

	"github.com/okteto/okteto/pkg/model"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

type fakeConfigRepo struct {
	err  error
	sha  string
	diff string
}

func (fcr fakeConfigRepo) GetSHA() (string, error)                   { return fcr.sha, fcr.err }
func (fcr fakeConfigRepo) GetLatestDirCommit(string) (string, error) { return fcr.sha, fcr.err }
func (fcr fakeConfigRepo) GetDiffHash(string) (string, error)        { return fcr.diff, fcr.err }

func Test_getBuildHashFromCommit(t *testing.T) {
	fs := afero.NewMemMapFs()
	err := afero.WriteFile(fs, "secret", []byte("bar"), 0600)
	assert.NoError(t, err)
	t.Setenv("BAR", "bar")
	type input struct {
		buildInfo *model.BuildInfo
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
				buildInfo: &model.BuildInfo{
					Args: model.BuildArgs{
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
					Secrets: model.BuildSecrets{
						"secret": "secret",
					},
					Context:    "context",
					Dockerfile: "dockerfile",
					Image:      "image",
				},
			},
			expected: "commit:1234567890;target:target;build_args:foo=bar;key=value;secrets:secret=secret;context:context;dockerfile:dockerfile;dockerfile_content:;diff:;image:image;",
		},
		{
			name: "invalid commit",
			input: input{
				repo: fakeConfigRepo{
					sha: "",
					err: assert.AnError,
				},
				buildInfo: &model.BuildInfo{
					Args: model.BuildArgs{
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
					Secrets: model.BuildSecrets{
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
				buildInfo: &model.BuildInfo{
					Args:   model.BuildArgs{},
					Target: "target",
					Secrets: model.BuildSecrets{
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
				buildInfo: &model.BuildInfo{
					Args: model.BuildArgs{
						{
							Name:  "foo",
							Value: "$BAR",
						},
					},
					Target: "target",
					Secrets: model.BuildSecrets{
						"secret": "secret",
					},
					Context:    "context",
					Dockerfile: "dockerfile",
					Image:      "image",
				},
			},
			expected: "commit:123;target:target;build_args:foo=bar;secrets:secret=secret;context:context;dockerfile:dockerfile;dockerfile_content:;diff:;image:image;",
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
