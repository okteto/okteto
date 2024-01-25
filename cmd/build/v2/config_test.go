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

package v2

import (
	"testing"

	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeConfigRegistry struct {
	err    error
	access bool
}

func (fcr fakeConfigRegistry) HasGlobalPushAccess() (bool, error) { return fcr.access, fcr.err }

type fakeConfigRepo struct {
	err     error
	sha     string
	url     string
	diff    string
	isClean bool
}

func (fcr fakeConfigRepo) GetSHA() (string, error)                   { return fcr.sha, fcr.err }
func (fcr fakeConfigRepo) IsClean() (bool, error)                    { return fcr.isClean, fcr.err }
func (fcr fakeConfigRepo) GetAnonymizedRepo() string                 { return fcr.url }
func (fcr fakeConfigRepo) GetLatestDirCommit(string) (string, error) { return fcr.sha, fcr.err }
func (fcr fakeConfigRepo) GetDiffHash(string) (string, error)        { return fcr.diff, fcr.err }

type fakeLogger struct{}

func (fl fakeLogger) Infof(format string, args ...interface{}) {}

func TestGetConfigStateless(t *testing.T) {
	type input struct {
		reg  fakeConfigRegistry
		repo fakeConfigRepo
	}
	tt := []struct {
		expected oktetoBuilderConfig
		name     string
		input    input
	}{
		{
			name: "global access clean commit",
			input: input{
				reg: fakeConfigRegistry{
					access: true,
					err:    nil,
				},
				repo: fakeConfigRepo{
					isClean: true,
					err:     nil,
				},
			},
			expected: oktetoBuilderConfig{
				hasGlobalAccess: true,
				isCleanProject:  true,
				repository: fakeConfigRepo{
					isClean: true,
					err:     nil,
				},
				fs:                  afero.NewOsFs(),
				isOkteto:            true,
				isSmartBuildsEnable: true,
			},
		},
		{
			name: "no global access clean commit",
			input: input{
				reg: fakeConfigRegistry{
					access: false,
					err:    nil,
				},
				repo: fakeConfigRepo{
					isClean: true,
					err:     nil,
				},
			},
			expected: oktetoBuilderConfig{
				hasGlobalAccess: false,
				isCleanProject:  true,
				repository: fakeConfigRepo{
					isClean: true,
					err:     nil,
				},
				fs:                  afero.NewOsFs(),
				isOkteto:            true,
				isSmartBuildsEnable: true,
			},
		},
		{
			name: "error on global access clean commit",
			input: input{
				reg: fakeConfigRegistry{
					access: false,
					err:    assert.AnError,
				},
				repo: fakeConfigRepo{
					isClean: true,
					err:     nil,
				},
			},
			expected: oktetoBuilderConfig{
				hasGlobalAccess: false,
				isCleanProject:  true,
				repository: fakeConfigRepo{
					isClean: true,
					err:     nil,
				},
				fs:                  afero.NewOsFs(),
				isOkteto:            true,
				isSmartBuildsEnable: true,
			},
		},
		{
			name: "error on clean commit and global access",
			input: input{
				reg: fakeConfigRegistry{
					access: false,
					err:    assert.AnError,
				},
				repo: fakeConfigRepo{
					isClean: false,
					err:     assert.AnError,
				},
			},
			expected: oktetoBuilderConfig{
				hasGlobalAccess: false,
				isCleanProject:  false,
				repository: fakeConfigRepo{
					isClean: false,
					err:     assert.AnError,
				},
				fs:                  afero.NewOsFs(),
				isOkteto:            true,
				isSmartBuildsEnable: true,
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			cfg := getConfigStateless(tc.input.reg, tc.input.repo, fakeLogger{}, true)
			assert.Equal(t, tc.expected, cfg)
		})
	}
}

func TestGetConfig(t *testing.T) {
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Namespace: "test",
				IsOkteto:  true,
			},
		},
		CurrentContext: "test",
	}
	type input struct {
		reg  fakeConfigRegistry
		repo fakeConfigRepo
	}
	tt := []struct {
		expected oktetoBuilderConfig
		name     string
		input    input
	}{
		{
			name: "global access clean commit",
			input: input{
				reg: fakeConfigRegistry{
					access: true,
					err:    nil,
				},
				repo: fakeConfigRepo{
					isClean: true,
					err:     nil,
				},
			},
			expected: oktetoBuilderConfig{
				hasGlobalAccess: true,
				isCleanProject:  true,
				repository: fakeConfigRepo{
					isClean: true,
					err:     nil,
				},
				fs:                  afero.NewOsFs(),
				isOkteto:            true,
				isSmartBuildsEnable: true,
			},
		},
		{
			name: "no global access clean commit",
			input: input{
				reg: fakeConfigRegistry{
					access: false,
					err:    nil,
				},
				repo: fakeConfigRepo{
					isClean: true,
					err:     nil,
				},
			},
			expected: oktetoBuilderConfig{
				hasGlobalAccess: false,
				isCleanProject:  true,
				repository: fakeConfigRepo{
					isClean: true,
					err:     nil,
				},
				fs:                  afero.NewOsFs(),
				isOkteto:            true,
				isSmartBuildsEnable: true,
			},
		},
		{
			name: "error on global access clean commit",
			input: input{
				reg: fakeConfigRegistry{
					access: false,
					err:    assert.AnError,
				},
				repo: fakeConfigRepo{
					isClean: true,
					err:     nil,
				},
			},
			expected: oktetoBuilderConfig{
				hasGlobalAccess: false,
				isCleanProject:  true,
				repository: fakeConfigRepo{
					isClean: true,
					err:     nil,
				},
				fs:                  afero.NewOsFs(),
				isOkteto:            true,
				isSmartBuildsEnable: true,
			},
		},
		{
			name: "error on clean commit and global access",
			input: input{
				reg: fakeConfigRegistry{
					access: false,
					err:    assert.AnError,
				},
				repo: fakeConfigRepo{
					isClean: false,
					err:     assert.AnError,
				},
			},
			expected: oktetoBuilderConfig{
				hasGlobalAccess: false,
				isCleanProject:  false,
				repository: fakeConfigRepo{
					isClean: false,
					err:     assert.AnError,
				},
				fs:                  afero.NewOsFs(),
				isOkteto:            true,
				isSmartBuildsEnable: true,
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			cfg := getConfig(tc.input.reg, tc.input.repo, fakeLogger{})
			assert.Equal(t, tc.expected, cfg)
		})
	}
}

func TestGetIsSmartBuildEnabled(t *testing.T) {
	tt := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "enabled feature flag",
			input:    "true",
			expected: true,
		},
		{
			name:     "disabled feature flag",
			input:    "false",
			expected: false,
		},
		{
			name:     "wrong feature flag value default true",
			input:    "falsess",
			expected: true,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(OktetoEnableSmartBuildEnvVar, tc.input)
			cfg := getIsSmartBuildEnabled()
			assert.Equal(t, tc.expected, cfg)
		})
	}
}

func TestGetGitCommit(t *testing.T) {
	tt := []struct {
		name     string
		expected string
		input    fakeConfigRepo
	}{
		{
			name: "valid commit",
			input: fakeConfigRepo{
				sha:     "1234567890",
				isClean: true,
				err:     nil,
			},
			expected: "1234567890",
		},
		{
			name: "invalid commit",
			input: fakeConfigRepo{
				sha:     "",
				isClean: true,
				err:     assert.AnError,
			},
			expected: "",
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			cfg := oktetoBuilderConfig{
				repository: tc.input,
			}
			assert.Equal(t, tc.expected, cfg.GetGitCommit())
		})
	}
}

func Test_GetAnonymizedRepo(t *testing.T) {
	cfg := oktetoBuilderConfig{
		repository: fakeConfigRepo{
			url: "repository url",
		},
	}

	require.Equal(t, "repository url", cfg.GetAnonymizedRepo())
}
