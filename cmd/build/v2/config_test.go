package v2

import (
	"testing"

	"github.com/okteto/okteto/pkg/model"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeConfigRegistry struct {
	access bool
	err    error
}

func (fcr fakeConfigRegistry) HasGlobalPushAccess() (bool, error) { return fcr.access, fcr.err }

type fakeConfigRepo struct {
	sha      string
	isClean  bool
	url      string
	treeHash string
	err      error
}

func (fcr fakeConfigRepo) GetSHA() (string, error)            { return fcr.sha, fcr.err }
func (fcr fakeConfigRepo) IsClean() (bool, error)             { return fcr.isClean, fcr.err }
func (fcr fakeConfigRepo) GetAnonymizedRepo() string          { return fcr.url }
func (fcr fakeConfigRepo) GetTreeHash(string) (string, error) { return fcr.treeHash, fcr.err }

func TestGetConfig(t *testing.T) {
	type input struct {
		reg  fakeConfigRegistry
		repo fakeConfigRepo
	}
	tt := []struct {
		name     string
		input    input
		expected oktetoBuilderConfig
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
				fs:       afero.NewOsFs(),
				isOkteto: true,
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
				fs:       afero.NewOsFs(),
				isOkteto: true,
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
				fs:       afero.NewOsFs(),
				isOkteto: true,
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
				fs:       afero.NewOsFs(),
				isOkteto: true,
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			cfg := getConfig(tc.input.reg, tc.input.repo)
			assert.Equal(t, tc.expected, cfg)
		})
	}
}

func TestGetGitCommit(t *testing.T) {
	tt := []struct {
		name     string
		input    fakeConfigRepo
		expected string
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

func TestGetBuildContextHash(t *testing.T) {
	cfg := oktetoBuilderConfig{
		repository: fakeConfigRepo{
			treeHash: "test",
		},
	}

	oktetoBuildHash := "9bb4ac6e28aaf8eb67e453cf9d593ac35e34c9766b92dd482b1833ff66ec49ca"
	buildInfo := &model.BuildInfo{
		Args:    model.BuildArgs{{Name: "testName", Value: "testValue"}},
		Secrets: model.BuildSecrets{"testNameSecret": "testValueSecret"},
	}
	require.Equal(t, oktetoBuildHash, cfg.GetBuildContextHash(buildInfo))
}
